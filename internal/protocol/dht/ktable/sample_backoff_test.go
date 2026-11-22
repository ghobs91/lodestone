package ktable

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

func testNode(t *testing.T) *node {
	t.Helper()

	tbl, ok := New(Params{NodeID: protocol.RandomNodeID()}).Table.(*table)
	if !ok {
		t.Fatal("New did not return a *table")
	}

	id := protocol.RandomNodeID()
	addr := netip.AddrPortFrom(netip.MustParseAddr("1.2.3.4"), 6881)

	tbl.PutNode(id, addr)

	n, ok := tbl.nodes.items[id]
	if !ok {
		t.Fatal("node was not added to the routing table")
	}

	// Clear the 5s "recently responded" gate so candidacy is decided only by
	// the backoff/BEP51 fields under test.
	n.lastRespondedAt = time.Now().Add(-time.Hour)

	return n
}

// A node that failed a sample query must be skipped, otherwise it keeps
// consuming the sample budget on every pass.
func TestSampleInfoHashesFailedDefersCandidate(t *testing.T) {
	n := testNode(t)

	if !n.IsSampleInfoHashesCandidate() {
		t.Fatal("fresh node should be a sample candidate")
	}

	n.apply(NodeSampleInfoHashesFailed(time.Now().Add(30 * time.Minute)))

	if n.IsSampleInfoHashesCandidate() {
		t.Fatal("node should not be a candidate while backed off")
	}
}

// The backoff is a delay, not a permanent exclusion.
func TestSampleInfoHashesFailedExpires(t *testing.T) {
	n := testNode(t)

	n.apply(NodeSampleInfoHashesFailed(time.Now().Add(-time.Second)))

	if !n.IsSampleInfoHashesCandidate() {
		t.Fatal("node should be a candidate again once the backoff has elapsed")
	}
}

// Nodes that answer with a BEP 51 protocol error are alive but unsupported; they
// must never be sampled again while they remain in the table.
func TestBep51UnsupportedIsNeverCandidate(t *testing.T) {
	n := testNode(t)

	n.apply(NodeBep51Support(false))

	if n.IsSampleInfoHashesCandidate() {
		t.Fatal("node without BEP 51 support should not be a sample candidate")
	}
}

// Dead addresses must not be re-fed into node discovery, otherwise the crawler
// burns its query budget re-probing peers that have already failed.
func TestFilterKnownAddrsSkipsFailedAddrs(t *testing.T) {
	tbl, ok := New(Params{NodeID: protocol.RandomNodeID()}).Table.(*table)
	if !ok {
		t.Fatal("New did not return a *table")
	}

	dead := netip.MustParseAddr("5.6.7.8")
	alive := netip.MustParseAddr("9.9.9.9")

	tbl.FailAddr(dead)

	got := tbl.FilterKnownAddrs([]netip.Addr{dead, alive})

	if len(got) != 1 || got[0] != alive {
		t.Fatalf("expected only the un-failed address, got %v", got)
	}
}

// A failed address must eventually be retried rather than excluded forever.
func TestFilterKnownAddrsRetriesAfterTTL(t *testing.T) {
	tbl := &table{
		addrs:       &reverseMap{addrs: map[string]*infoForAddr{}},
		failedAddrs: expirable.NewLRU[netip.Addr, struct{}](10, nil, 20*time.Millisecond),
	}

	addr := netip.MustParseAddr("5.6.7.9")
	tbl.FailAddr(addr)

	if got := tbl.FilterKnownAddrs([]netip.Addr{addr}); len(got) != 0 {
		t.Fatalf("expected failed address to be filtered, got %v", got)
	}

	time.Sleep(60 * time.Millisecond)

	if got := tbl.FilterKnownAddrs([]netip.Addr{addr}); len(got) != 1 {
		t.Fatalf("expected address to be retried after the TTL, got %v", got)
	}
}

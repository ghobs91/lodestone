package search

import (
	"testing"

	"github.com/ghobs91/lodestone/internal/database/query"
)

// TestContentTypeFacetSupportsGroupBy verifies the content-type facet opts into
// single-query GROUP BY aggregation and maps NULL values to the "null" key.
func TestContentTypeFacetSupportsGroupBy(t *testing.T) {
	facet := TorrentContentTypeFacet()

	gb, ok := facet.(query.FacetGroupBy)
	if !ok {
		t.Fatal("content type facet should implement query.FacetGroupBy")
	}

	if got := gb.NullValue(); got != "null" {
		t.Fatalf("NullValue = %q, want %q", got, "null")
	}
}

// TestNonGroupByFacetFallsBack verifies facets without a GROUP BY field are not
// treated as grouped, so they keep using the per-value counting path.
func TestNonGroupByFacetFallsBack(t *testing.T) {
	facet := TorrentTagsFacet()

	if _, ok := facet.(query.FacetGroupBy); ok {
		t.Fatal("torrent tag facet should not implement query.FacetGroupBy")
	}
}

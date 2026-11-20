package concurrency

import (
	"testing"
	"time"
)

// receiveBatch waits for one batch from the channel, failing the test if the
// batching goroutine stalls instead of emitting it.
func receiveBatch[T any](t *testing.T, ch BatchingChannel[T]) []T {
	t.Helper()

	select {
	case batch := <-ch.Out():
		return batch
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a batch")

		return nil
	}
}

// TestBatchingChannelSizeTriggeredFlushRepeats guards against a regression
// where the batching timer was stopped by a size-triggered flush but never
// restarted. The next size-triggered flush then blocked forever draining a
// timer channel that would never fire again, wedging every consumer of the
// channel (the DHT crawler's node discovery and infohash triage among them).
func TestBatchingChannelSizeTriggeredFlushRepeats(t *testing.T) {
	// A very long max wait time ensures only size-triggered flushes occur, so
	// the timer is never restarted by the time-based path.
	ch := NewBatchingChannel[int](10, 3, time.Hour)

	for i := 0; i < 6; i++ {
		ch.In() <- i
	}

	for flush := 0; flush < 2; flush++ {
		batch := receiveBatch(t, ch)
		if len(batch) != 3 {
			t.Fatalf("flush %d: expected batch size 3, got %d", flush, len(batch))
		}
	}
}

// TestBatchingChannelTimeTriggeredFlushRepeats verifies that partial batches
// keep being emitted on the timer interval.
func TestBatchingChannelTimeTriggeredFlushRepeats(t *testing.T) {
	ch := NewBatchingChannel[int](10, 100, 10*time.Millisecond)

	for flush := 0; flush < 2; flush++ {
		ch.In() <- flush

		batch := receiveBatch(t, ch)
		if len(batch) != 1 || batch[0] != flush {
			t.Fatalf("flush %d: unexpected batch %v", flush, batch)
		}
	}
}

// TestBatchingChannelDrainsOnClose verifies that closing the input emits the
// remaining buffered items before the output is closed.
func TestBatchingChannelDrainsOnClose(t *testing.T) {
	ch := NewBatchingChannel[int](10, 100, time.Hour)

	ch.In() <- 1
	ch.In() <- 2
	close(ch.In())

	select {
	case batch := <-ch.Out():
		if len(batch) != 2 {
			t.Fatalf("expected a batch of 2, got %v", batch)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the final batch")
	}
}

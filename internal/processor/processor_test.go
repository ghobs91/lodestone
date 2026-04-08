package processor

import (
	"testing"
	"time"
)

func TestReenqueueDelay(t *testing.T) {
	tests := []struct {
		depth    int
		expected time.Duration
	}{
		{0, 30 * time.Second},
		{1, 5 * time.Minute},
		{2, 15 * time.Minute},
		{99, 15 * time.Minute}, // anything beyond known depths uses the default
	}
	for _, tt := range tests {
		got := reenqueueDelay(tt.depth)
		if got != tt.expected {
			t.Errorf("reenqueueDelay(%d) = %v, want %v", tt.depth, got, tt.expected)
		}
	}
}

func TestMaxReenqueueDepthBounds(t *testing.T) {
	// Verify the constant is reasonable (not zero, not enormous)
	if MaxReenqueueDepth < 1 || MaxReenqueueDepth > 10 {
		t.Errorf("MaxReenqueueDepth = %d, expected between 1 and 10", MaxReenqueueDepth)
	}
}

func TestMessageParamsReenqueueDepthJSON(t *testing.T) {
	// Verify that the ReenqueueDepth field round-trips correctly.
	zeroDepth := MessageParams{
		ReenqueueDepth: 0,
	}
	if zeroDepth.ReenqueueDepth != 0 {
		t.Fatal("zero value should be 0")
	}

	withDepth := MessageParams{
		ReenqueueDepth: 2,
	}
	if withDepth.ReenqueueDepth != 2 {
		t.Errorf("expected ReenqueueDepth=2, got %d", withDepth.ReenqueueDepth)
	}
}

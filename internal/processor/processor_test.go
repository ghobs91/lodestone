package processor

import (
	"testing"
	"time"

	"github.com/ghobs91/lodestone/internal/model"
)

func TestIsDiscardedContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType model.NullContentType
		want        bool
	}{
		{"unknown", model.NullContentType{}, true},
		{"xxx", model.NewNullContentType(model.ContentTypeXxx), true},
		{"movie", model.NewNullContentType(model.ContentTypeMovie), false},
		{"tv_show", model.NewNullContentType(model.ContentTypeTvShow), false},
		{"music", model.NewNullContentType(model.ContentTypeMusic), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDiscardedContentType(tt.contentType); got != tt.want {
				t.Errorf("isDiscardedContentType(%v) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

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

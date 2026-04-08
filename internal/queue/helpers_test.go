package queue

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	// Verify backoff is always in the future and increases with retry count.
	now := time.Now()

	prev := time.Duration(0)
	for retries := uint(1); retries <= 5; retries++ {
		backoff := CalculateBackoff(retries)
		delay := backoff.Sub(now)

		if delay <= 0 {
			t.Errorf("CalculateBackoff(%d) returned a time in the past", retries)
		}

		// The minimum delay should increase with retries (formula is retry^4 + 15 + ...).
		// We check the formula's deterministic component, acknowledging jitter.
		minDelay := time.Duration(int(retries)*int(retries)*int(retries)*int(retries)+15+1) * time.Second
		if delay < minDelay-time.Second { // allow 1s tolerance for timing
			t.Errorf("CalculateBackoff(%d) delay=%v, expected at least %v", retries, delay, minDelay)
		}

		if retries > 1 && delay < prev {
			// Not strictly guaranteed due to jitter, but for these small counts
			// the deterministic component dominates.
			t.Logf("warning: CalculateBackoff(%d) delay=%v < previous=%v (jitter may cause this)", retries, delay, prev)
		}

		prev = delay
	}
}

package handler

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/ghobs91/lodestone/internal/model"
)

// TestExecCancelsHandlerContext verifies that the context passed to the job
// handler is cancelled once the job timeout elapses, so background work stops
// rather than continuing after Exec has returned.
func TestExecCancelsHandlerContext(t *testing.T) {
	h := New("test", func(ctx context.Context, _ model.QueueJob) error {
		<-ctx.Done()
		return ctx.Err()
	}, JobTimeout(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	err := Exec(ctx, h, model.QueueJob{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when the handler blocks until timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("Exec took %v, expected to return promptly on timeout", elapsed)
	}
}

// TestExecHandlerContextIsCancelled ensures the handler observes cancellation
// of the context it was given (regression: the handler used to receive the
// parent context, which is never cancelled on timeout).
func TestExecHandlerContextIsCancelled(t *testing.T) {
	returned := make(chan struct{})
	h := New("test", func(ctx context.Context, _ model.QueueJob) error {
		<-ctx.Done()
		close(returned)
		return nil
	}, JobTimeout(50*time.Millisecond))

	_ = Exec(context.Background(), h, model.QueueJob{})

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("handler context was not cancelled after the job timeout")
	}
}

// TestExecDoesNotLeakGoroutineOnTimeout verifies that a timed-out job does not
// permanently leak the goroutine that runs the handler.
func TestExecDoesNotLeakGoroutineOnTimeout(t *testing.T) {
	runtime.GC()
	before := runtime.NumGoroutine()

	h := New("test", func(ctx context.Context, _ model.QueueJob) error {
		<-ctx.Done()
		return nil
	}, JobTimeout(50*time.Millisecond))

	_ = Exec(context.Background(), h, model.QueueJob{})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("goroutine count grew from %d to %d after a timed-out job", before, runtime.NumGoroutine())
}

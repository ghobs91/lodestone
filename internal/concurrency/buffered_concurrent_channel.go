package concurrency

import (
	"context"
	"sync"
)

type BufferedConcurrentChannel[T any] interface {
	In() chan<- T
	Run(context.Context, func(T)) error
}

func NewBufferedConcurrentChannel[T any](capacity int, concurrency int) BufferedConcurrentChannel[T] {
	return &bufferedConcurrentChannel[T]{
		ch:          make(chan T, capacity),
		concurrency: concurrency,
	}
}

type bufferedConcurrentChannel[T any] struct {
	ch          chan T
	concurrency int
}

func (ch *bufferedConcurrentChannel[T]) In() chan<- T {
	return ch.ch
}

func (ch *bufferedConcurrentChannel[T]) Run(ctx context.Context, f func(T)) error {
	// Pre-spawn a fixed pool of worker goroutines that process items from the
	// channel. This avoids the overhead of creating a new goroutine per item.
	var wg sync.WaitGroup
	for i := 0; i < ch.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case next, ok := <-ch.ch:
					if !ok {
						return
					}
					f(next)
				}
			}
		}()
	}

	// Wait for context cancellation; then close the channel to signal workers.
	<-ctx.Done()
	close(ch.ch)
	wg.Wait()

	return ctx.Err()
}

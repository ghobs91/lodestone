package concurrency

import (
	"time"
)

type BatchingChannel[T any] interface {
	In() chan<- T
	Out() <-chan []T
}

type batchingChannel[T any] struct {
	input        chan T
	output       chan []T
	buffer       []T
	maxBatchSize int
	maxWaitTime  time.Duration
	timer        *time.Timer
}

func NewBatchingChannel[T any](capacity int, maxBatchSize int, maxWaitTime time.Duration) BatchingChannel[T] {
	ch := &batchingChannel[T]{
		input:        make(chan T, capacity),
		output:       make(chan []T, 1),
		maxBatchSize: maxBatchSize,
		maxWaitTime:  maxWaitTime,
		timer:        time.NewTimer(maxWaitTime),
	}
	go ch.batch()

	return ch
}

func (ch *batchingChannel[T]) In() chan<- T {
	return ch.input
}

func (ch *batchingChannel[T]) Out() <-chan []T {
	return ch.output
}

func (ch *batchingChannel[T]) batch() {
	defer close(ch.output)

	for {
		select {
		case next, ok := <-ch.input:
			if !ok {
				// Drain remaining buffer before exiting.
				if len(ch.buffer) > 0 {
					ch.flushLocked()
				}
				return
			}

			ch.buffer = append(ch.buffer, next)
			if len(ch.buffer) >= ch.maxBatchSize {
				// Stop the timer early — we're flushing now due to size.
				if !ch.stopTimer() {
					// Timer already fired, drain the channel so we don't
					// get a spurious wake-up on the next select iteration.
					<-ch.timer.C
				}
				ch.flushLocked()
			}
		case <-ch.timer.C:
			if len(ch.buffer) > 0 {
				ch.flushLocked()
			}
			// Restart the timer for the next interval.
			ch.resetTimer()
		}
	}
}

// stopTimer stops the timer and returns true if the call actually stopped it.
// Returns false if the timer has already fired (and the channel needs draining).
func (ch *batchingChannel[T]) stopTimer() bool {
	if !ch.timer.Stop() {
		return false
	}
	return true
}

// resetTimer resets the timer for another maxWaitTime interval.
// Must only be called after the timer channel has been drained or the timer was stopped.
func (ch *batchingChannel[T]) resetTimer() {
	ch.timer.Reset(ch.maxWaitTime)
}

func (ch *batchingChannel[T]) flushLocked() {
	batch := ch.buffer
	ch.buffer = nil
	ch.output <- batch
}

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
		output:       make(chan []T, 4),
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
					ch.flush()
				}
				return
			}

			ch.buffer = append(ch.buffer, next)
			if len(ch.buffer) >= ch.maxBatchSize {
				ch.stopAndDrainTimer()
				ch.flush()
				// Restart the timer so later size-triggered flushes can keep
				// stopping it safely, and partial batches still flush on time.
				ch.timer.Reset(ch.maxWaitTime)
			}
		case <-ch.timer.C:
			if len(ch.buffer) > 0 {
				ch.flush()
			}
			// Restart the timer for the next interval.
			ch.timer.Reset(ch.maxWaitTime)
		}
	}
}

// stopAndDrainTimer stops the timer and drains its channel without blocking.
// A blocking drain is unsafe: Stop returns false both when the timer has just
// fired and when it was already stopped and drained, so a blocking receive
// would hang the batching goroutine forever in the latter case.
func (ch *batchingChannel[T]) stopAndDrainTimer() {
	if !ch.timer.Stop() {
		select {
		case <-ch.timer.C:
		default:
		}
	}
}

func (ch *batchingChannel[T]) flush() {
	batch := ch.buffer
	ch.buffer = nil
	ch.output <- batch
}

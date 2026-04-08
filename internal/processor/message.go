package processor

import (
	"github.com/ghobs91/lodestone/internal/classifier"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

const MessageName = "process_torrent"

type ClassifyMode int

const (
	// ClassifyModeDefault will use any pre-existing content match as a hint
	// This is the most common use case and will only attempt to match previously unmatched torrents
	ClassifyModeDefault ClassifyMode = iota
	// ClassifyModeRematch will ignore any pre-existing classification and always classify from scratch
	// This is useful if there are errors in matches from an earlier version that need to be corrected
	ClassifyModeRematch
)

// MaxReenqueueDepth is the maximum number of times failed hashes from a
// partially-successful batch may be re-enqueued as a new job. Once this depth
// is reached the remaining failures are simply dropped (the queue-level retry
// mechanism still applies to each individual job).
const MaxReenqueueDepth = 2

type MessageParams struct {
	ClassifyMode       ClassifyMode     `json:"ClassifyMode,omitempty"`
	ClassifierWorkflow string           `json:"ClassifierWorkflow,omitempty"`
	ClassifierFlags    classifier.Flags `json:"ClassifierFlags,omitempty"`
	InfoHashes         []protocol.ID    `json:"InfoHashes"`
	// ReenqueueDepth tracks how many times this job's failed hashes have been
	// split off and re-enqueued. Zero for the original job.
	ReenqueueDepth int `json:"ReenqueueDepth,omitempty"`
}

func NewQueueJob(msg MessageParams, options ...model.QueueJobOption) (model.QueueJob, error) {
	return model.NewQueueJob(
		MessageName,
		msg,
		append([]model.QueueJobOption{model.QueueJobMaxRetries(2)}, options...)...,
	)
}

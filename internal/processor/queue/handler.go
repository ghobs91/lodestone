package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ghobs91/lodestone/internal/classifier"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/processor"
	"github.com/ghobs91/lodestone/internal/queue/handler"
	"github.com/ghobs91/lodestone/internal/settings"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Processor lazy.Lazy[processor.Processor]
	Settings  lazy.Lazy[settings.ClassifierSettingsStore]
}

type Result struct {
	fx.Out
	Handler lazy.Lazy[handler.Handler] `group:"queue_handlers"`
}

func New(p Params) Result {
	return Result{
		Handler: lazy.New(func() (handler.Handler, error) {
			pr, err := p.Processor.Get()
			if err != nil {
				return handler.Handler{}, err
			}
			settingsStore, err := p.Settings.Get()
			if err != nil {
				return handler.Handler{}, err
			}
			return handler.New(
				processor.MessageName,
				func(ctx context.Context, job model.QueueJob) (err error) {
					msg := &processor.MessageParams{}
					if err := json.Unmarshal([]byte(job.Payload), msg); err != nil {
						return err
					}

					// Merge stored classifier settings as runtime flags (job-level flags take precedence).
					cs, settingsErr := settingsStore.Get(ctx)
					if settingsErr == nil {
						if msg.ClassifierFlags == nil {
							msg.ClassifierFlags = make(classifier.Flags)
						}
						if _, ok := msg.ClassifierFlags["delete_xxx"]; !ok {
							msg.ClassifierFlags["delete_xxx"] = cs.DeleteXxx
						}
					}

					return pr.Process(ctx, *msg)
				},
				handler.JobTimeout(time.Second*60*10),
				handler.Concurrency(1),
			), nil
		}),
	}
}

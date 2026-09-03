package dhtcrawlerhealthcheck

import (
	"context"
	"errors"
	"time"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/health"
	"github.com/ghobs91/lodestone/internal/protocol/dht/server"
	"go.uber.org/zap"
)

func NewCheck(
	dhtCrawlerActive *concurrency.AtomicValue[bool],
	lastResponses *concurrency.AtomicValue[server.LastResponses],
	logger *zap.SugaredLogger,
) health.Check {
	return health.Check{
		Name: "dht",
		IsActive: func() bool {
			return dhtCrawlerActive.Get()
		},
		Timeout:            time.Second,
		MaxTimeInError:     time.Minute, // Allow up to 1 minute of errors before marking down
		MaxContiguousFails: 3,           // Allow up to 3 consecutive failures before marking down
		Check: func(context.Context) error {
			lr := lastResponses.Get()
			if lr.StartTime.IsZero() {
				logger.Debugw("dht health check: server not started yet")
				return nil
			}
			now := time.Now()
			sinceStart := now.Sub(lr.StartTime)
			sinceLastSuccess := now.Sub(lr.LastSuccess)
			sinceLastResponse := now.Sub(lr.LastResponse)

			if lr.LastSuccess.IsZero() {
				if sinceStart < 30*time.Second {
					logger.Debugw("dht health check: warming up, no response yet",
						"sinceStart", sinceStart,
						"sinceLastResponse", sinceLastResponse,
						"lastError", lr.LastError)
					return nil
				}
				logger.Warnw("dht health check: no response within 30 seconds",
					"sinceStart", sinceStart,
					"sinceLastResponse", sinceLastResponse,
					"lastError", lr.LastError)
				return errors.New("no response within 30 seconds")
			}
			if sinceLastSuccess > time.Minute {
				logger.Warnw("dht health check: no successful responses within last minute",
					"sinceStart", sinceStart,
					"sinceLastSuccess", sinceLastSuccess,
					"sinceLastResponse", sinceLastResponse,
					"lastError", lr.LastError)
				return errors.New("no successful responses within last minute")
			}
			return nil
		},
	}
}

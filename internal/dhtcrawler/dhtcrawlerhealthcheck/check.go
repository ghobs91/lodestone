package dhtcrawlerhealthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/health"
	"github.com/ghobs91/lodestone/internal/protocol/dht/server"
	"go.uber.org/zap"
)

func NewCheck(
	dhtCrawlerActive *concurrency.AtomicValue[bool],
	lastResponses *concurrency.AtomicValue[server.LastResponses],
	remoteHealthURL string,
	logger *zap.SugaredLogger,
) health.Check {
	return health.Check{
		Name: "dht",
		IsActive: func() bool {
			// Active when the crawler runs locally, or when we are configured to
			// report a remote crawler's health (the web process).
			return dhtCrawlerActive.Get() || remoteHealthURL != ""
		},
		Timeout:            time.Second * 5,
		MaxTimeInError:     time.Minute, // Allow up to 1 minute of errors before marking down
		MaxContiguousFails: 3,           // Allow up to 3 consecutive failures before marking down
		Check: func(ctx context.Context) error {
			if !dhtCrawlerActive.Get() {
				return checkRemote(ctx, remoteHealthURL)
			}
			return checkLocal(lastResponses, logger)
		},
	}
}

// checkRemote probes the crawler's /status endpoint (when the crawler runs in a
// separate process/container) and reports the remote "dht" check status.
func checkRemote(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build remote dht health request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote dht health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Details map[string]struct {
			Status string `json:"status"`
		} `json:"details"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return fmt.Errorf("decode remote dht health: %w", decodeErr)
	}

	dht, ok := result.Details["dht"]
	if !ok {
		return errors.New("remote dht health: no dht check reported")
	}
	if dht.Status != "up" {
		return fmt.Errorf("remote dht status: %s", dht.Status)
	}

	return nil
}

func checkLocal(lastResponses *concurrency.AtomicValue[server.LastResponses], logger *zap.SugaredLogger) error {
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
}

package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// requesterLazy defers instantiation of the requester (and possible failure) until the first request is made,
// avoiding failure when the TMDB client is not needed.
type requesterLazy struct {
	once      sync.Once
	config    Config
	logger    *zap.SugaredLogger
	err       error
	requester Requester
}

func (r *requesterLazy) Request(
	ctx context.Context,
	path string,
	queryParams map[string]string,
	result interface{},
) (*resty.Response, error) {
	r.once.Do(func() {
		r.requester, r.err = newRequester(ctx, r.config, r.logger)
	})

	if r.err != nil {
		return nil, r.err
	}

	return r.requester.Request(ctx, path, queryParams, result)
}

func newRequester(ctx context.Context, config Config, logger *zap.SugaredLogger) (Requester, error) {
	if !config.Enabled {
		return nil, errors.New("TMDB is disabled")
	}

	if config.APIKey == defaultTmdbAPIKey {
		logger.Warnln(
			"you are using the default TMDB api key; TMDB requests will be limited to 1 per second; " +
				"to remove this warning please configure a personal TMDB api key",
		)

		config.RateLimit = time.Second
		config.RateLimitBurst = 8
	}

	r := requesterLogger{
		requester: requesterFailFast{
			requester: requesterSemaphore{
				requester: requesterLimiter{
					requester: requester{
						resty: resty.New().
							SetBaseURL(config.BaseURL).
							SetQueryParam("api_key", config.APIKey).
							SetRetryCount(2).
							SetRetryWaitTime(5 * time.Second).
							SetRetryMaxWaitTime(30 * time.Second).
							SetTimeout(10 * time.Second).
							EnableTrace().
							SetLogger(logger).
							AddRetryCondition(func(r *resty.Response, _ error) bool {
								return r != nil && r.StatusCode() == http.StatusTooManyRequests
							}).
							SetRetryAfter(func(_ *resty.Client, r *resty.Response) (time.Duration, error) {
								if r == nil {
									return 0, nil
								}
								if retryAfter := r.Header().Get("Retry-After"); retryAfter != "" {
									if secs, err := strconv.Atoi(retryAfter); err == nil {
										return time.Duration(secs) * time.Second, nil
									}
									if t, err := http.ParseTime(retryAfter); err == nil {
										if d := time.Until(t); d > 0 {
											return d, nil
										}
									}
								}
								return 0, nil
							}),
					},

					limiter: rate.NewLimiter(rate.Every(config.RateLimit), config.RateLimitBurst),
				},
				semaphore: semaphore.NewWeighted(2),
			},
			isUnauthorized: &concurrency.AtomicValue[bool]{},
		},
		logger: logger,
	}

	err := client{r}.ValidateAPIKey(ctx)
	if errors.Is(err, ErrUnauthorized) {
		if config.APIKey == defaultTmdbAPIKey {
			logger.Errorw("default TMDB API key is invalid; TMDB features will be unavailable")
			return r, fmt.Errorf("default api key is invalid: %w", err)
		}

		logger.Errorw("configured TMDB API key is invalid; TMDB features will be unavailable", "error", err)
		return r, fmt.Errorf("configured api key is invalid: %w", err)
	}

	return r, err
}

package metainforequester

import (
	"time"

	"golang.org/x/time/rate"
)

type Config struct {
	RequestTimeout         time.Duration
	KeyMutexSize           int
	GlobalRequestRateLimit rate.Limit // 0 disables the global limit
}

func NewDefaultConfig() Config {
	return Config{
		RequestTimeout: 6 * time.Second,
		KeyMutexSize:   1000,
		// Cap total outbound metadata requests to avoid overwhelming the
		// conntrack table and ephemeral port range with TCP connections.
		GlobalRequestRateLimit: rate.Limit(50),
	}
}

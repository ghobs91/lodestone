package server

import (
	"time"

	"golang.org/x/time/rate"
)

type Config struct {
	Port                 uint16
	QueryTimeout         time.Duration
	GlobalQueryRateLimit rate.Limit // 0 disables the global limit
}

func NewDefaultConfig() Config {
	return Config{
		Port:         3334,
		QueryTimeout: time.Second * 4,
		// Cap total outbound DHT queries to avoid saturating the conntrack table
		// and starving other Docker services of network connectivity.
		GlobalQueryRateLimit: rate.Limit(100),
	}
}

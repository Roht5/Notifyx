package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewClient parses redisURL (Upstash uses the rediss:// scheme, which go-redis
// recognises and connects to over TLS automatically) and verifies connectivity
// with a Ping before returning, mirroring postgres.NewPool's startup check.
func NewClient(ctx context.Context, redisURL string, log *logger.Logger) (*redis.Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Infow("connected to Redis", "addr", opts.Addr)
	return client, nil
}

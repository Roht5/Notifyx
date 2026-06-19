package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewPool creates and validates a pgxpool connection pool.
//
// pgxpool is Go's idiomatic PostgreSQL pool — it manages a set of reusable
// connections so the app doesn't open a new TCP connection on every query.
// The pool is safe for concurrent use across goroutines.
//
// maxConns/minConns are set explicitly rather than left to pgxpool's own default
// (max(4, NumCPU()) — not huge, but still implicit and host-dependent). Free-tier
// Postgres plans cap total connections low, so an explicit, predictable pool size here
// matters more than squeezing out extra concurrency.
func NewPool(ctx context.Context, databaseURL string, maxConns, minConns int32, log *logger.Logger) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Ping verifies the pool can actually reach the database.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.Infow("connected to PostgreSQL", "host", cfg.ConnConfig.Host)
	return pool, nil
}

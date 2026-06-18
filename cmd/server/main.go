package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rohit-bagade/notifyx/config"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

func main() {
	// 1. Load configuration from environment / .env file.
	cfg, err := config.Load()
	if err != nil {
		// We cannot use Zap yet because the logger hasn't been initialised.
		// os.Exit(1) is the idiomatic way to bail out in main() before any
		// deferred functions matter.
		println("failed to load config:", err.Error())
		os.Exit(1)
	}

	// 2. Initialise structured logger.
	log, err := logger.New(cfg.Env)
	if err != nil {
		println("failed to init logger:", err.Error())
		os.Exit(1)
	}
	defer log.Sync()

	log.Infow("starting Notifyx", "env", cfg.Env, "port", cfg.Port)

	// 3. Run database migrations (applies any pending .up.sql files).
	if err := postgres.RunMigrations(cfg.DatabaseURL, "migrations", log); err != nil {
		log.Fatal("migration failed", "error", err)
	}

	// 4. Open PostgreSQL connection pool.
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, log)
	if err != nil {
		log.Fatal("database connection failed", "error", err)
	}
	defer pool.Close()

	log.Infow("Notifyx ready — waiting for shutdown signal")

	// 5. Block until SIGINT or SIGTERM (Ctrl-C or docker stop).
	// This will be replaced in Phase 2 when we start the HTTP server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Infow("shutdown signal received, exiting")
}

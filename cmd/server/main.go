package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rohit-bagade/notifyx/config"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/api/routes"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		println("failed to load config:", err.Error())
		os.Exit(1)
	}

	log, err := logger.New(cfg.Env)
	if err != nil {
		println("failed to init logger:", err.Error())
		os.Exit(1)
	}
	defer log.Sync()

	log.Infow("starting Notifyx", "env", cfg.Env, "port", cfg.Port)

	if err := postgres.RunMigrations(cfg.DatabaseURL, "migrations", log); err != nil {
		log.Fatal("migration failed", "error", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, log)
	if err != nil {
		log.Fatal("database connection failed", "error", err)
	}
	defer pool.Close()

	// Build repositories.
	tenantRepo := postgres.NewTenantRepository(pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(pool)
	channelRepo := postgres.NewTenantChannelRepository(pool)
	rateLimitRepo := postgres.NewTenantRateLimitRepository(pool)

	// Build handlers.
	h := &routes.Handlers{
		Health: handlers.NewHealthHandler(),
		Tenant: handlers.NewTenantHandler(&handlers.TenantService{
			Tenants:    tenantRepo,
			APIKeys:    apiKeyRepo,
			Channels:   channelRepo,
			RateLimits: rateLimitRepo,
		}, log),
	}

	// Wire up Echo with all routes.
	e := routes.Setup(h, apiKeyRepo, log)

	// Start HTTP server in a goroutine so we can listen for shutdown signals.
	// A goroutine is a lightweight concurrent function — think of it as a background thread.
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", "error", err)
		}
	}()

	log.Infow("Notifyx ready", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Infow("shutdown signal received, draining connections...")

	// Give in-flight requests 10 seconds to complete before hard-stopping.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Errorw("server shutdown error", "error", err)
	}

	log.Infow("Notifyx stopped")
}

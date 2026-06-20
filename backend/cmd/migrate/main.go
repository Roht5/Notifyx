// Command migrate runs golang-migrate against DATABASE_URL and exits.
// Intended for Render's Pre-Deploy Command, decoupling schema migrations
// from the web service's own startup path (see decisions.md, "migrations-on-startup").
package main

import (
	"os"

	"github.com/rohit-bagade/notifyx/config"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		println("failed to load config:", err.Error())
		os.Exit(1)
	}

	log, err := logger.New(cfg.Env, cfg.LogLevel)
	if err != nil {
		println("failed to init logger:", err.Error())
		os.Exit(1)
	}
	defer log.Sync()

	if err := postgres.RunMigrations(cfg.DatabaseURL, "migrations", log); err != nil {
		log.Fatal("migration failed", "error", err)
	}

	log.Infow("migrations applied")
}

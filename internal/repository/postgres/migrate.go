package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// Register the pgx/v5 driver for golang-migrate.
	// Blank imports like this are a Go pattern for side-effect registration —
	// the package's init() function registers the driver without us needing
	// to call anything explicitly.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// RunMigrations applies all pending up-migrations from the migrations/ directory.
// It is safe to call on every startup — if there is nothing to apply, it returns nil.
func RunMigrations(databaseURL string, migrationsPath string, log *logger.Logger) error {
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required for migrations")
	}

	// golang-migrate's pgx/v5 driver expects the "pgx5://" scheme prefix.
	// We replace the standard "postgres://" prefix to match what the driver expects.
	connStr := databaseURL
	switch {
	case len(databaseURL) > 11 && databaseURL[:11] == "postgresql://":
		connStr = "pgx5://" + databaseURL[13:]
	case len(databaseURL) > 11 && databaseURL[:11] == "postgres://":
		connStr = "pgx5://" + databaseURL[11:]
	}

	m, err := migrate.New("file://"+migrationsPath, connStr)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		// ErrNoChange means all migrations are already applied — not an error.
		if errors.Is(err, migrate.ErrNoChange) {
			log.Infow("migrations: no new migrations to apply")
			return nil
		}
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Infow("migrations: applied successfully")
	return nil
}

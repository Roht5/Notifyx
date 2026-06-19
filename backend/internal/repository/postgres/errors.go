package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned when a query finds no matching row.
// Handlers should map this to HTTP 404.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned when an insert/update violates a unique constraint.
// Handlers should map this to HTTP 409.
var ErrAlreadyExists = errors.New("already exists")

// pgUniqueViolation is the Postgres error code for a unique constraint violation.
const pgUniqueViolation = "23505"

// isUniqueViolation reports whether err is a Postgres unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

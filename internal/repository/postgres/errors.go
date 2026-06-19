package postgres

import "errors"

// ErrNotFound is returned when a query finds no matching row.
// Handlers should map this to HTTP 404.
var ErrNotFound = errors.New("not found")

# Go Testing Skill

Use this skill whenever writing tests for any Notifyx component. Ensures consistent test structure, proper mocking patterns, and meaningful coverage across all phases.

## When to use

- Writing unit tests for any `internal/` package
- Writing integration tests against real PostgreSQL or Redis
- After implementing any handler, consumer, repository, or service

## Test file conventions

- Test file lives next to the file it tests: `notification.go` → `notification_test.go`
- Package name: use `package X_test` (black-box) for handlers and consumers, `package X` (white-box) for internal logic
- One `TestXxx` function per behaviour, not per function
- Use table-driven tests for multiple input/output cases

```go
func TestRateLimiter_Allow(t *testing.T) {
    tests := []struct {
        name      string
        requests  int
        wantAllow bool
    }{
        {"under limit", 5, true},
        {"at limit", 10, true},
        {"over limit", 11, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // arrange
            // act
            // assert
        })
    }
}
```

## Mocking pattern

Define interfaces in the consumer package, implement fakes in `_test.go` files. Never use a mocking library — hand-written fakes are clearer.

```go
// In internal/kafka/consumers/email_test.go
type fakeEmailSender struct {
    sentTo []string
    err    error
}

func (f *fakeEmailSender) Send(to, subject, body string) error {
    f.sentTo = append(f.sentTo, to)
    return f.err
}
```

## Repository tests (integration)

Repository tests hit a **real PostgreSQL** instance (use docker-compose locally). Never mock the DB layer.

```go
func TestNotificationRepository_Create(t *testing.T) {
    db := testhelper.NewTestDB(t)         // spins up test DB, runs migrations, cleans up on t.Cleanup
    repo := postgres.NewNotificationRepository(db)

    n := &domain.Notification{ /* ... */ }
    err := repo.Create(context.Background(), n)
    require.NoError(t, err)
    require.NotEmpty(t, n.ID)
}
```

## Redis tests (integration)

Use a real Upstash Redis or local Redis in docker-compose. Never mock Redis — sliding window logic must be tested against real Redis behaviour.

## Kafka consumer tests (unit)

Test the handler function directly — don't spin up a real Kafka broker in unit tests. Pass a fake message payload and assert on the side effects (DB writes, delivery status updates).

## Test helpers

Put shared test utilities in `internal/testhelper/`:
- `NewTestDB(t *testing.T) *sql.DB` — creates isolated test DB, runs migrations, registers cleanup
- `NewTestLogger() *zap.Logger` — returns a no-op Zap logger for tests
- `FixtureNotification(overrides ...func(*domain.Notification)) *domain.Notification` — builds test data

## Assertions

Use `github.com/stretchr/testify` — `require` for fatal assertions, `assert` for non-fatal:

```go
require.NoError(t, err)          // stops test on failure
assert.Equal(t, want, got)       // continues test on failure
assert.ErrorIs(t, err, ErrX)     // check error type
```

## Coverage targets per component

| Component | Target | Notes |
|---|---|---|
| Repository (postgres/) | 80%+ | Integration tests against real DB |
| Rate limiter | 90%+ | Core correctness guarantee |
| Deduplication | 90%+ | Core correctness guarantee |
| Kafka consumers | 70%+ | Unit test handler logic |
| API handlers | 70%+ | Test request validation + response shape |
| Channel integrations | 60%+ | Mock the provider SDK |
| Scheduler | 80%+ | Test cron trigger + fired flag logic |

## What NOT to test

- Config loading (it's just `os.Getenv`)
- Zap logger setup
- Echo framework routing (it works — test your handlers, not Echo)
- Generated code

## Running tests

```bash
go test ./...                          # all tests
go test ./internal/ratelimit/...       # specific package
go test -run TestRateLimiter ./...     # specific test
go test -race ./...                    # race condition detection (run before every commit)
go test -cover ./...                   # coverage report
```

Always run with `-race` before committing. Race conditions in Kafka consumers and WebSocket handlers are a real risk.

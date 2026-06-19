# Code Review Skill

Source: anthropics/claude-code plugins/code-review + plugins/feature-dev/agents/code-reviewer (adapted for Notifyx)

Use this skill after completing any phase or significant set of changes. Reviews code for bugs, security issues, and convention adherence using confidence-based filtering.

## When to use

- After completing any phase in `planning/milestones.md`
- Before committing a large set of changes
- When something feels off but you can't identify it

## Confidence Scoring

Rate every potential issue 0–100. **Only report issues ≥ 80.**

| Score | Meaning |
|---|---|
| 0–25 | False positive or pre-existing — skip |
| 50 | Real but minor — skip unless critical path |
| 75 | Likely real and important |
| **80+** | **Report this** |
| 100 | Definitely broken — fix immediately |

## Review Checklist for Notifyx

### Correctness
- [ ] Error paths return errors — never silently swallow them
- [ ] Kafka consumer failures trigger retry logic, not silent drops
- [ ] DB writes are transactional where multiple tables are written together
- [ ] Redis operations handle connection errors (don't panic on Redis failure)
- [ ] Goroutines have proper lifecycle management (no leaked goroutines)
- [ ] Context cancellation is respected in long-running operations

### Security
- [ ] No credentials, API keys, or secrets hardcoded anywhere
- [ ] API key is hashed (bcrypt) before storage — never stored plaintext
- [ ] SQL queries use parameterized statements — no string concatenation
- [ ] Tenant ID always validated — never trust tenant from request body alone
- [ ] No sensitive data in log lines (emails, phone numbers, API keys)

### Go Conventions
- [ ] Errors are wrapped with context: `fmt.Errorf("doing X: %w", err)`
- [ ] Interfaces are defined in the consumer package, not the provider
- [ ] No `init()` functions with side effects
- [ ] Struct fields exported only when necessary
- [ ] No global mutable state outside of explicitly initialized singletons

### Notifyx Architecture
- [ ] SQL queries are in `internal/repository/postgres/` — not in handlers or consumers
- [ ] Config values come from `config/config.go` — no hardcoded values
- [ ] Kafka topic names match `requirements/architecture.md` — no invented topic names
- [ ] Redis key names match `requirements/architecture.md` — no invented key patterns
- [ ] Zap logger used everywhere — no `fmt.Println` or standard `log` package

### Performance
- [ ] No N+1 queries (bulk fetch, not loop-fetch)
- [ ] Redis operations batched where possible (PIPELINE)
- [ ] No blocking calls inside Kafka consumer hot path

## Output Format

```
## Code Review — [component name]

### Critical (fix before proceeding)
- [confidence: 95] internal/api/handlers/notify.go:42 — SQL query uses string concat, SQL injection risk. Use parameterized query.

### Important (fix soon)
- [confidence: 82] internal/kafka/consumers/email.go:78 — Error swallowed silently on Resend failure. Should return err to trigger retry.

### Passed
- Error handling: consistent
- Logging: Zap used throughout
- Architecture: matches requirements/architecture.md
```

If no issues ≥ 80: write "No high-confidence issues found. Code meets Notifyx standards."

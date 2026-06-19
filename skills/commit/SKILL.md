# Commit Skill

Source: anthropics/claude-code plugins/commit-commands (adapted for Notifyx)

Use this skill after completing any phase or significant unit of work to create a clean, well-scoped git commit.

## When to use

- After completing a phase from `planning/milestones.md`
- After completing a self-contained unit of work (e.g. "all DB migrations", "Kafka producer + consumer base")
- Never commit broken or half-finished work

## Pre-commit checklist

Before committing, verify:
- [ ] Code compiles: `cd backend && go build ./...`
- [ ] No hardcoded credentials or API keys
- [ ] Zap logger used — no `fmt.Println`
- [ ] Config values from `config/config.go` — nothing hardcoded
- [ ] `planning/milestones.md` checkboxes updated for completed tasks

## Commit message format

```
<type>(<scope>): <short summary>

<optional body — what and why, not how>
```

**Types:**
- `feat` — new functionality
- `fix` — bug fix
- `refactor` — restructuring without behaviour change
- `test` — adding or updating tests
- `chore` — migrations, config, tooling

**Scope** = the phase or component (e.g. `foundation`, `kafka`, `email-channel`, `ratelimit`, `flutter-dashboard`)

**Examples:**
```
feat(foundation): add PostgreSQL connection pool and migration runner

feat(kafka): add producer and per-channel consumer base with retry + DLQ

feat(email-channel): integrate Resend for email delivery with receipt tracking

fix(ratelimit): correct sliding window expiry calculation for global cap
```

## Steps

1. `git status` — confirm only intended files are changed
2. Stage specific files — never `git add .` blindly (avoid accidental `.env` commits)
3. `cd backend && go build ./...` — confirm it compiles
4. Commit with message following format above
5. Update `planning/milestones.md` checkboxes if not done yet

## Never commit

- `.env` files or any file containing real credentials
- Generated binaries
- Half-finished implementations that break compilation
- Debug `fmt.Println` statements

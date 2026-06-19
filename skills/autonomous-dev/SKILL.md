# Autonomous Development Skill

Source: anthropics/claude-code plugins/ralph-wiggum (adapted for Notifyx)

Use this skill to work through one or more phases autonomously without stopping for confirmation at every step. Designed for well-defined phases with clear success criteria.

## When to use

- When asked to "build Phase X autonomously" or "complete Phase X end to end"
- For phases that are purely mechanical with no design decisions left (e.g. Phase 1 Foundation, Phase 9 Scheduler)
- NOT for phases requiring architectural decisions or external credentials setup

## The Loop Pattern

```
Read phase tasks → Implement → Verify → Fix if broken → Commit → Move to next task
```

Each iteration:
1. Pick the next unchecked task from `planning/milestones.md`
2. Use the **feature-dev skill** to implement it (phases 1–5)
3. Use the **code-review skill** to check it (phase 6)
4. Fix any issues found
5. Run `go build ./...` — if it fails, fix before proceeding
6. Use the **commit skill** to commit
7. Check the task off in `planning/milestones.md`
8. Repeat

## Completion criteria

Stop and report to the developer when:
- All tasks in the specified phase are checked off in `planning/milestones.md`
- OR you hit a blocker that requires external input (credentials, design decision, unclear requirement)
- OR `go build ./...` fails and you cannot identify the fix within 2 attempts

## Blocker protocol

When blocked, do NOT guess or invent a solution. Instead:
1. Stop the loop
2. State exactly what you were trying to do
3. State exactly what the blocker is (missing credential, unclear requirement, compilation error)
4. Ask a specific question — not "what should I do?" but "should X be Y or Z?"

## Phases safe for autonomous execution

| Phase | Safe? | Notes |
|---|---|---|
| 1 — Foundation | ✅ | Pure setup, no design decisions |
| 2 — Tenant & Auth | ✅ | Schema + CRUD, well-defined |
| 3 — Kafka Infrastructure | ✅ | Well-specified in architecture.md |
| 4 — Notification Core | ✅ | Well-specified |
| 5 — Rate Limiting & Dedup | ✅ | Redis patterns fully defined |
| 6 — Channel Integrations | ⚠️ | Needs real API credentials |
| 7 — WebSocket | ✅ | Well-specified |
| 8 — Templates | ✅ | Simple CRUD + rendering |
| 9 — Scheduler & Cleanup | ✅ | Well-specified |
| 10 — Analytics | ✅ | Queries against existing schema |
| 11 — Observability | ✅ | Additive, non-breaking |
| 12 — Flutter Dashboard | ⚠️ | UI requires visual verification |
| 13 — Deployment | ⚠️ | Needs Render account + env vars |

## Token efficiency rules

- Read a file once — don't re-read files you've already read in this session unless it changed
- Don't explain code you just wrote unless it contains a non-obvious Go pattern
- Don't summarize what you're about to do — just do it
- Batch independent file writes in parallel tool calls
- Skip phases 1–3 of feature-dev skill for tasks under 20 lines of code

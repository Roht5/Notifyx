# Feature Development Skill

Source: anthropics/claude-code plugins/feature-dev (adapted for Notifyx)

Use this skill when implementing any phase from `planning/task-breakdown.md`. It enforces a structured 7-phase approach that prevents premature coding and reduces wasted tokens.

## When to use

- Starting any new phase (Phase 1 through Phase 13)
- Implementing a specific task within a phase
- When uncertain how a new component fits into the existing codebase

## The 7 Phases

### Phase 1 — Discovery
Before writing any code:
- Re-read the relevant section in `planning/task-breakdown.md`
- Re-read `requirements/architecture.md` for the component you're building
- Re-read `planning/decisions.md` for any decisions that constrain this work
- State clearly: what you're building, what it connects to, what it depends on

### Phase 2 — Codebase Exploration
Explore existing code before adding new code:
- Use Grep to find existing patterns (error handling, logging, DB queries, Kafka usage)
- Read related files to understand conventions already established
- Map: where does this new component fit in the folder structure?
- Never invent conventions — match what already exists

### Phase 3 — Clarifying Questions
Before designing, identify anything underspecified:
- Edge cases not covered in requirements
- Integration points with other components
- Error handling expectations
- Ask the developer if genuinely blocked — otherwise make a reasonable call and note it

### Phase 4 — Architecture Design
Design before implementing:
- Describe the component: files to create, interfaces, data flow
- Identify what calls this component and what it calls
- No code yet — just the plan in plain English + file paths

### Phase 5 — Implementation
Only after phases 1–4 are complete:
- Follow Go conventions established in existing code
- Match error handling, logging (Zap), and DB patterns already in the codebase
- One file at a time — fully complete each file before moving to the next
- Reference `requirements/architecture.md` for schema, route, and Redis key details

### Phase 6 — Quality Review
After implementation, review your own work:
- Does it match the architecture described in Phase 4?
- Are errors handled and logged correctly?
- Is it consistent with existing patterns?
- Would this pass the code-review skill? (run it if unsure)

### Phase 7 — Summary
After completing the work:
- List files created/modified
- Note any decisions made that aren't in `planning/decisions.md`
- Update checkbox in `planning/milestones.md` for completed tasks
- State what phase/task comes next

## Notifyx-specific rules

- **Go is new to the developer** — add a short comment explaining non-obvious Go idioms
- Always use Zap for logging — never `fmt.Println` or `log.Printf`
- Always check `requirements/architecture.md` for Redis key names, Kafka topic names, and DB schema before writing code — don't invent them
- Config values always come from `config/config.go` — never hardcode URLs, credentials, or ports
- All DB queries go in `internal/repository/postgres/` — never write SQL in handlers

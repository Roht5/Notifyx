# Notifyx — Agent Handoff Document

## What is this?

Notifyx is a multi-tenant distributed notification service built as a portfolio project. It supports Email, Push, SMS, and In-app (WebSocket) channels via an event-driven pipeline using Kafka.

**Current state: Phase 1 (Foundation) and Phase 2 (Tenant & Auth) complete. Phase 3 (Kafka Infrastructure) is next.**

---

## About the Developer

- **Name:** Rohit Bagade
- **Role:** Software Engineer at Zenoti (Sept 2024 – Present), 1.6 years experience
- **Background:** Backend-focused. Extensive experience with .NET 8 microservices, Kafka, Redis, PostgreSQL, Azure Service Bus, Azure Container Apps, AWS S3, distributed systems, event-driven architecture, multi-tenancy
- **Go experience:** None — this project is his first Go codebase. Explain Go-specific patterns when introducing them
- **Flutter experience:** Strong — he built the Flutter Web frontend at Zenoti. Flutter dashboard is not a learning exercise
- **Goal:** Build a production-grade portfolio project that showcases distributed systems skills for SDE-2 backend roles

---

## Folder Structure

```
Notifyx/
├── docs/
│   └── HANDOFF.md           ← you are here
├── planning/
│   ├── milestones.md        ← 13 phases with checkbox task lists
│   ├── dev-order.md         ← why each phase comes in its order + dependency map
│   ├── task-breakdown.md    ← detailed implementation tasks per phase (start here when coding)
│   └── decisions.md         ← all decisions made and why
├── requirements/
│   ├── overview.md          ← project goals
│   ├── tech-stack.md        ← all tools and providers
│   ├── functional-requirements.md
│   ├── non-functional-requirements.md
│   └── architecture.md      ← flows, schema, API routes, Kafka topics, Redis keys, folder structure
├── skills/                  ← reusable skill docs (read before starting any phase)
├── backend/                 ← Go service (cd here before running Go commands)
│   ├── cmd/server/          ← entrypoint
│   ├── config/              ← env-based config
│   ├── internal/            ← all application code
│   ├── pkg/                 ← shared packages (logger, telemetry)
│   ├── migrations/          ← versioned SQL migrations
│   ├── go.mod
│   └── .env.example
└── frontend/
    └── flutter/             ← Flutter Web dashboard (Phase 12)
```

---

## How to Read This

1. Start with `requirements/overview.md` for the big picture
2. Read `requirements/architecture.md` for the full technical design (schema, flows, API routes)
3. Read `planning/decisions.md` to understand what was decided and why — don't relitigate these
4. Use `planning/task-breakdown.md` as the implementation guide — it tells you exactly what to build in each phase
5. Use `planning/milestones.md` to track progress (checkboxes)

---

## Key Rules (Non-negotiable)

- **Free tier only** — no paid services. Every external dependency must have a free tier
- **Single deployable** — one Go binary, not microservices
- **HTML before LaTeX** — if updating resume files, always update HTML first (unrelated to this project but noted for context)
- **Go is new to the developer** — explain Go idioms, don't assume prior knowledge
- **Flutter dashboard is desktop-only** — no responsive/mobile layout for now
- **No fallback channels** — if email fails, it does not retry via push or SMS
- **Update HTML before LaTeX** when making resume changes

---

## Tech Stack Summary

| Layer | Choice |
|---|---|
| Backend | Go 1.22, Echo framework |
| WebSockets | Gorilla WebSocket |
| Database | PostgreSQL (Render free tier) |
| Cache | Upstash Redis (free tier) |
| Message Broker | Confluent Kafka (free tier, 5GB/month) |
| Email | Resend (100/day free) |
| Push | Firebase FCM (free) |
| SMS | Fast2SMS (free tier, India) |
| Logging | Uber Zap |
| Metrics | Prometheus + Grafana |
| Tracing | OpenTelemetry |
| Frontend | Flutter Web + Bloc |
| Deployment | Render + Docker Compose locally |

---

## Available Skills

Read the relevant SKILL.md before starting any significant work. Skills save tokens by giving you a repeatable process.

| Skill | When to use |
|---|---|
| `skills/feature-dev/SKILL.md` | Starting any new phase or component — enforces 7-phase structured approach |
| `skills/code-review/SKILL.md` | After completing a phase — confidence-scored review checklist for Go + Notifyx patterns |
| `skills/commit/SKILL.md` | After completing a unit of work — commit format, pre-commit checklist, what never to commit |
| `skills/autonomous-dev/SKILL.md` | When asked to complete a phase autonomously — loop pattern, blocker protocol, token efficiency rules |
| `skills/go-testing/SKILL.md` | Writing any test — table-driven patterns, mock conventions, coverage targets per component |
| `skills/api-design/SKILL.md` | Writing any Echo handler, middleware, or route — consistent structure, error format, auth pattern |
| `skills/migrations/SKILL.md` | Writing or running any DB migration — file naming, up/down rules, golang-migrate usage |
| `skills/debugging/SKILL.md` | When stuck on any error — systematic layer-by-layer diagnosis for Go, Kafka, Redis, WebSocket |
| `skills/docker/SKILL.md` | Phase 13 or any Docker/local dev issue — Dockerfile, docker-compose, render.yaml templates |
| `skills/webapp-testing/SKILL.md` | After building any Flutter dashboard screen — Playwright testing with Flutter Web-specific notes |

### Recommended flow for each phase
```
Read feature-dev/SKILL.md → Implement → Read code-review/SKILL.md → Fix issues → Read commit/SKILL.md → Commit
```

### Recommended flow for autonomous execution
```
Read autonomous-dev/SKILL.md → Loop: [feature-dev → code-review → commit] until phase complete
```

---

## Where to Start

**Phase 3 — Kafka Infrastructure** (see `planning/task-breakdown.md` → Phase 3):
- Phases 1 and 2 are complete — see `planning/milestones.md` for what's done.
- All Go work happens inside `backend/` — run `cd backend` before any `go` commands.
- Read `planning/task-breakdown.md` Phase 3 section before writing any Kafka code.

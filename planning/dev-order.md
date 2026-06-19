# Notifyx — Development Order & Reasoning

## Why this order?

Each phase builds on the previous one. Nothing is built in isolation — every phase produces something runnable or testable before moving on.

---

## 1. Foundation first
No code works without config, DB, and models. Migrations define the schema everything else depends on. Domain models are the shared language across all layers.

## 2. Tenant & Auth before notifications
Every notification is scoped to a tenant. Auth middleware must exist before any protected route is built. Building this early means all subsequent routes are properly secured from day one.

## 3. Kafka infrastructure before channel integrations
The consumer loop, retry logic, and DLQ wiring are the same regardless of channel. Build the generic worker infrastructure once, then plug in channel providers. This avoids rewriting retry logic 4 times.

## 4. Notification core with stub delivery
Wire up the full send pipeline (API → rate limit → dedup → DB → Kafka) before any real provider is integrated. This lets you test the entire flow end-to-end with fake delivery, so when real providers are added the pipeline is already proven.

## 5. Rate limiting & dedup before going live
These are correctness guarantees, not nice-to-haves. Must be in place before real sends happen to avoid duplicate or runaway notifications.

## 6. Channel integrations one at a time
Each channel is independent. Complete one fully (including delivery receipts) before starting the next. Suggested order: Email → Push → SMS → In-app (In-app last because it depends on the WebSocket layer).

## 7. WebSocket after other channels
In-app is the most complex channel (stateful connections, presence, offline queue). Building it after the other channels means the consumer pattern is well understood.

## 8. Templates after channels are working
Templates are a layer on top of the send pipeline. Adding them once the pipeline is stable is cleaner than building them upfront.

## 9. Scheduler & cleanup after core send works
Scheduled notifications reuse the exact same Kafka → consumer pipeline. The cron runner is a thin layer on top. Cleanup is a simple cron job, low risk, added late.

## 10. Analytics after data exists
Analytics queries need real notification data in the DB to be meaningful. Build and test after the send pipeline has produced data.

## 11. Observability woven in throughout, formalized here
Zap logging should be added as each component is built. This phase formalizes Prometheus metrics and OTel tracing across all layers and sets up the Grafana dashboard.

## 12. Flutter dashboard last
The dashboard is a consumer of the API. Building it last means the API is stable and all endpoints are available to wire up. No risk of frontend blocking on missing API endpoints.

## 13. Deployment last
Dockerize and deploy once everything works locally. Avoids debugging deployment issues while the application is still changing rapidly.

---

## Dependency Map

```
Foundation
    └── Tenant & Auth
            └── Kafka Infrastructure
                    └── Notification Core
                            └── Rate Limiting & Dedup
                                    └── Channel Integrations
                                                └── WebSocket & In-app
                                                └── Templates
                                                └── Scheduler & Cleanup
                                                └── Analytics
                                                        └── Observability (runs alongside all)
                                                        └── Flutter Dashboard
                                                                └── Deployment
```

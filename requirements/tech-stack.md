# Notifyx — Tech Stack

## Backend
| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.22 | |
| Framework | Echo | Clean, fast, great middleware support |
| WebSockets | Gorilla WebSocket | Most mature Go WS library |

## Data
| Layer | Choice | Notes |
|---|---|---|
| Database | PostgreSQL | Render free tier |
| Cache | Upstash Redis | Free tier — rate limiting, dedup, presence |
| Message Broker | Confluent Kafka | Free tier (5GB/month) |

## Notification Channels
| Channel | Provider | Notes |
|---|---|---|
| Email | Resend | 100 emails/day free |
| Push | Firebase FCM | Free |
| SMS | Fast2SMS | Free tier, India |
| In-app | WebSocket (Gorilla) | Real-time delivery |

## Observability
| Tool | Purpose |
|---|---|
| Uber Zap | Structured logging |
| Prometheus | Metrics |
| OpenTelemetry | Distributed tracing |
| Grafana | Metrics dashboard (local / Render) |

## Frontend
| Layer | Choice |
|---|---|
| Framework | Flutter Web |
| State Management | Bloc |

## Deployment
| Target | Tool |
|---|---|
| Production | Render (1 Web Service + 1 PostgreSQL) |
| Local Dev | Docker Compose |
| External Services | Confluent Kafka, Upstash Redis |

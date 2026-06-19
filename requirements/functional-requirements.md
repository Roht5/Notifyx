# Notifyx — Functional Requirements

## Notification Sending
- Send notifications via Email, Push, SMS, or In-app channels
- Support immediate send and scheduled send (future datetime)
- Support batch send (one notification to multiple recipients)
- Support priority levels: critical, high, normal, low — affects queue ordering
- Support notification templates with variable placeholders (e.g. `{{name}}`, `{{orderId}}`)

## Scheduling
- Accept a `scheduled_at` datetime on any notification
- Internal cron runner polls DB every 30s and publishes due notifications to Kafka
- Scheduled notifications follow the same delivery pipeline as immediate ones

## Deduplication
- Each notification request can include an `idempotency_key`
- If the same key is received within 24 hours, the request is ignored and original response returned
- Implemented via Redis with 24h TTL

## Rate Limiting
- Per tenant per channel: sliding window (configurable max per minute)
- Per tenant global cap: sliding window across all channels
- When limit is hit: notification is queued for later delivery (not dropped, not errored)
- Implemented via Upstash Redis

## Retry & DLQ
- On delivery failure: retry up to 3 times with exponential backoff
- After 3 failures: publish to single DLQ Kafka topic
- DLQ consumer stores failed messages in PostgreSQL `dlq_messages` table

## Multi-tenancy
- Each tenant has a unique ID and API key
- Tenants can opt in/out of specific channels
- Per-tenant rate limit configuration
- Super-admin role can manage all tenants
- All data is scoped to tenant
- Flutter dashboard is open (no login required) for now — auth to be added later

## Fallback Channels
- No fallback between channels — if push fails, it does not retry via email or any other channel
- Each channel fails independently and follows its own retry → DLQ path

## In-app / WebSocket
- Clients connect via `WS /ws/connect?tenantId=&userId=`
- Presence tracking: Redis tracks online/offline per user
- Offline delivery: if user is offline, notification stored in Redis offline queue and flushed on reconnect

## Delivery Receipts
- Track delivery status per notification per channel: pending → queued → delivered / failed
- Track number of attempts
- Track delivery timestamp

## Notification History
- Store all notifications in PostgreSQL
- 90-day retention — `expires_at` set at insert time (created_at + 90 days)
- Internal cleanup job (cron, daily) hard-deletes rows where `expires_at < now()`

## Templates
- Create, update, delete templates per tenant
- Templates have: name, channel, subject (email), body with `{{variable}}` placeholders
- Rendered at send time with provided variables

## Analytics (per tenant)
- Total sent, delivered, failed
- Delivery rate per channel
- DLQ trend over time
- Channel breakdown

# Debugging Skill

Use this skill when stuck — compilation errors, runtime panics, Kafka/Redis connection failures, unexpected behaviour. Gives a systematic diagnosis process to avoid burning tokens on guesswork.

## When to use

- `go build ./...` fails
- A test fails unexpectedly
- Kafka consumer isn't receiving messages
- Redis operation returns wrong result
- WebSocket connection drops immediately
- Handler returns 500 but logs are empty

---

## Step 1 — Read the full error

Never truncate or paraphrase an error message when diagnosing. Read every word.

Key signals:
- `undefined: X` → missing import or wrong package name
- `cannot use X as type Y` → type mismatch, check interface compliance
- `no required module provides` → run `go mod tidy`
- `nil pointer dereference` → something wasn't initialised — trace back to where it's created
- `context deadline exceeded` → DB or Redis connection timeout — check env vars and connectivity
- `KAFKA_ERROR` → check broker address in config, check Confluent free tier limits

---

## Step 2 — Isolate the layer

Notifyx has clear layers. Identify which layer the error is in:

```
Request → Handler → Service → Repository → DB
                 → Rate Limiter → Redis
                 → Kafka Producer → Kafka
                              → Consumer → Channel Provider
```

- If the error is in the handler: check request binding and validation
- If the error is in the repository: run the SQL directly against the DB to verify
- If the error is in Redis: check the key with `redis-cli GET <key>`
- If the error is in Kafka: check consumer group lag in Confluent dashboard
- If the error is in a channel provider: check the provider's API response body, not just the status code

---

## Step 3 — Check config first

80% of connection errors are wrong config values. Before debugging code, verify:

```bash
# Print all resolved config values at startup (add to main.go temporarily)
log.Info("config loaded",
    zap.String("db_url", cfg.DatabaseURL),
    zap.String("redis_url", cfg.RedisURL),
    zap.String("kafka_brokers", cfg.KafkaBrokers),
)
```

Common mistakes:
- `DATABASE_URL` missing `?sslmode=disable` for local dev
- `REDIS_URL` missing `redis://` prefix
- Kafka broker address is `localhost:9092` locally but Confluent cloud URL in production

---

## Step 4 — Kafka specific

**Consumer not receiving messages:**
1. Check topic name matches exactly — `notifyx.email` not `notifyx-email`
2. Check consumer group ID is set and consistent
3. Check Confluent free tier hasn't hit the 5GB limit
4. Add a log line at the top of the consumer loop to confirm it's running
5. Publish a test message manually via Confluent UI

**Messages stuck in DLQ:**
1. Read the `error` field in `dlq_messages` table — it tells you exactly what failed
2. Check if the channel provider API key is set in config
3. Check if the provider's free tier limit is hit (Resend: 100/day, Fast2SMS: check dashboard)

---

## Step 5 — Redis specific

**Sliding window not working:**
1. Check the key exists: `redis-cli ZRANGEBYSCORE rate:{tenantId}:{channel} 0 +inf`
2. Check TTL: `redis-cli TTL rate:{tenantId}:{channel}`
3. The key format must exactly match `requirements/architecture.md`

**Presence not updating:**
1. Check WebSocket disconnect handler is being called — add a log line
2. Check `presence:{tenantId}:{userId}` exists: `redis-cli EXISTS presence:{tid}:{uid}`

---

## Step 6 — WebSocket specific

**Connection drops immediately:**
1. Check the upgrade request has correct headers (`Upgrade: websocket`, `Connection: Upgrade`)
2. Check `tenantId` and `userId` query params are present
3. Check auth middleware isn't blocking the WS endpoint (WS uses query param auth, not header)

**Messages not delivered:**
1. Check presence key exists in Redis
2. Check connection is in the connection registry (add a log after registration)
3. Check the in-app Kafka consumer is running (add a startup log)

---

## Step 7 — Go compilation specific

**Interface not satisfied:**
```
cannot use *EmailSender as type channels.Sender
```
→ Run: `go vet ./...` — it'll tell you which method is missing or has wrong signature

**Import cycle:**
```
import cycle not allowed
```
→ The dependency goes `A → B → A`. Fix by: extracting the shared type to `internal/domain/`, or using an interface to invert the dependency.

**Module issues:**
```bash
go mod tidy          # add missing, remove unused
go mod verify        # check module integrity
go clean -modcache   # nuclear option if modules are corrupted
```

---

## Blocker escalation

If after following all steps above you're still stuck after 2 attempts:
1. Stop — do not keep trying random things
2. Write exactly: what you tried, what error you got, what you expected
3. Ask a specific question to the developer
4. Do not modify unrelated code while waiting

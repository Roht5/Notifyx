# Database Migrations Skill

Use this skill when writing or running PostgreSQL migrations for Notifyx. Ensures migrations are safe, reversible, and consistently structured.

## When to use

- Writing any new migration file in `migrations/`
- Adding a column, index, or table
- Running migrations locally or on Render
- Debugging a migration failure

## Tool: golang-migrate

Notifyx uses `golang-migrate`. Each migration is two files:

```
migrations/
  001_create_tenants.up.sql
  001_create_tenants.down.sql
  002_create_tenant_channels.up.sql
  002_create_tenant_channels.down.sql
  ...
```

**Naming:** `{number}_{description}.{up|down}.sql`
- Number is zero-padded 3 digits: `001`, `002`, ..., `009`
- Description is snake_case, describes the change
- `.up.sql` = apply the migration
- `.down.sql` = reverse it

## Running migrations

```bash
# Apply all pending migrations
migrate -path ./migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate -path ./migrations -database "$DATABASE_URL" down 1

# Check current version
migrate -path ./migrations -database "$DATABASE_URL" version
```

In Go code (called at startup in `main.go`):

```go
m, err := migrate.New("file://migrations", cfg.DatabaseURL)
if err != nil {
    log.Fatal("migration init failed", zap.Error(err))
}
if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
    log.Fatal("migration failed", zap.Error(err))
}
```

## Migration rules

**Always write a down migration.** Even if you think you'll never roll back — write it. It documents the inverse operation.

**Never modify an existing migration.** Once a migration is committed and run, it is immutable. To change the schema, write a new migration.

**One change per migration.** Don't combine `CREATE TABLE` + `ALTER TABLE` + `CREATE INDEX` in one file. Separate concerns = easier rollback.

**Use explicit column types.** Never use `TEXT` when `VARCHAR(255)` is appropriate. Be specific.

**All timestamps are `TIMESTAMPTZ`.** Never plain `TIMESTAMP` — timezone-aware is non-negotiable.

**UUID primary keys.** Use `UUID DEFAULT gen_random_uuid()` — requires `pgcrypto` extension (add in migration 000 if not present).

## Standard table template

```sql
-- up
CREATE TABLE IF NOT EXISTS table_name (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_table_name_tenant_id ON table_name(tenant_id);

-- down
DROP TABLE IF EXISTS table_name;
```

## Notifyx migration order

Must be applied in this order (foreign key dependencies):

```
001 → tenants
002 → tenant_channels        (FK: tenants)
003 → tenant_rate_limits     (FK: tenants)
004 → api_keys               (FK: tenants)
005 → notification_templates (FK: tenants)
006 → notifications          (FK: tenants, notification_templates)
007 → notification_deliveries (FK: notifications)
008 → scheduled_notifications (FK: notifications)
009 → dlq_messages           (FK: notifications)
```

## `expires_at` for 90-day retention

The `notifications` table has `expires_at TIMESTAMPTZ`. Set it at insert time:

```sql
expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '90 days')
```

Index it for the cleanup job:

```sql
CREATE INDEX idx_notifications_expires_at ON notifications(expires_at);
```

## JSONB for metadata

```sql
metadata JSONB DEFAULT '{}'::jsonb
```

Query example:
```sql
SELECT * FROM notifications WHERE metadata->>'source' = 'api';
```

## Common mistakes

- Forgetting `IF NOT EXISTS` on CREATE — will fail if run twice
- Using `TIMESTAMP` instead of `TIMESTAMPTZ`
- Missing index on foreign key columns — causes slow JOINs
- Writing a down migration that doesn't fully reverse the up — test by running up then down then up again
- Committing a migration that hasn't been tested locally

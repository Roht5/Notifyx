CREATE TABLE IF NOT EXISTS tenants (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_channels (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_tenant_channels_tenant_id ON tenant_channels(tenant_id);

CREATE TABLE IF NOT EXISTS tenant_rate_limits (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel      TEXT        NOT NULL,
    max_per_min  INT         NOT NULL DEFAULT 60,
    global_cap   INT         NOT NULL DEFAULT 300,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_tenant_rate_limits_tenant_id ON tenant_rate_limits(tenant_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_hash   TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash  ON api_keys(key_hash);

CREATE TABLE IF NOT EXISTS notification_templates (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    channel    TEXT        NOT NULL,
    subject    TEXT,
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_notification_templates_tenant_id ON notification_templates(tenant_id);

CREATE TABLE IF NOT EXISTS notifications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel          TEXT        NOT NULL,
    priority         TEXT        NOT NULL DEFAULT 'normal',
    status           TEXT        NOT NULL DEFAULT 'pending',
    recipient_id     TEXT,
    recipient_email  TEXT,
    recipient_phone  TEXT,
    recipient_token  TEXT,
    template_id      UUID        REFERENCES notification_templates(id) ON DELETE SET NULL,
    subject          TEXT,
    body             TEXT,
    metadata         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key  TEXT,
    scheduled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '90 days'),
    CONSTRAINT notifications_tenant_idempotency_key_key UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id       ON notifications(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notifications_status          ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_channel         ON notifications(channel);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at      ON notifications(created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_expires_at      ON notifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel         TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'queued',
    attempts        INT         NOT NULL DEFAULT 0,
    error_message   TEXT,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_notification_id ON notification_deliveries(notification_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status           ON notification_deliveries(status);

CREATE TABLE IF NOT EXISTS scheduled_notifications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    scheduled_at    TIMESTAMPTZ NOT NULL,
    fired           BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_notifications_fired_at
    ON scheduled_notifications(scheduled_at)
    WHERE fired = FALSE;

CREATE TABLE IF NOT EXISTS dlq_messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel         TEXT        NOT NULL,
    error           TEXT        NOT NULL,
    attempts        INT         NOT NULL DEFAULT 0,
    last_tried_at   TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dlq_messages_notification_id ON dlq_messages(notification_id);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_created_at      ON dlq_messages(created_at);

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
    idempotency_key  TEXT        UNIQUE,
    scheduled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '90 days')
);

CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id       ON notifications(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notifications_status          ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_channel         ON notifications(channel);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at      ON notifications(created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_expires_at      ON notifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);

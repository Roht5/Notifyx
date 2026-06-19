CREATE TABLE IF NOT EXISTS notification_templates (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    channel    TEXT        NOT NULL,
    subject    TEXT,
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_notification_templates_tenant_id ON notification_templates(tenant_id);

CREATE TABLE IF NOT EXISTS tenant_channels (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_tenant_channels_tenant_id ON tenant_channels(tenant_id);

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

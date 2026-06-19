ALTER TABLE notifications DROP CONSTRAINT notifications_idempotency_key_key;
ALTER TABLE notifications ADD CONSTRAINT notifications_tenant_idempotency_key_key UNIQUE (tenant_id, idempotency_key);

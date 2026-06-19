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

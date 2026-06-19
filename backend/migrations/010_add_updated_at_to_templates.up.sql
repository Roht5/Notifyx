ALTER TABLE notification_templates
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

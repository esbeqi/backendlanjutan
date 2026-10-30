-- Notifications are addressed to enrolled user accounts in this project.
-- Keep historical rows and enforce one notification per event and recipient.
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_outbox_unique;

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_outbox_recipient_unique;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_outbox_recipient_unique
    UNIQUE (outbox_id, user_id);

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'sent';

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS processing_until TIMESTAMPTZ;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS last_error TEXT;

ALTER TABLE notifications
    ALTER COLUMN sent_at SET DEFAULT NOW();

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_status_valid;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_status_valid
    CHECK (status IN ('pending','processing','sent','failed'));

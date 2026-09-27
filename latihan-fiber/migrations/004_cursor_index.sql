-- Urutan column pada index harus sama dengan ORDER BY
-- pada query cursor.
CREATE INDEX IF NOT EXISTS users_created_at_id_desc_idx
ON users (created_at DESC, id DESC);
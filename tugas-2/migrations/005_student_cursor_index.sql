CREATE INDEX IF NOT EXISTS idx_students_cursor
ON students (created_at DESC, id DESC);
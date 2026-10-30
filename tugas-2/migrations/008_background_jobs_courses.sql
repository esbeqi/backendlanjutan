CREATE TABLE IF NOT EXISTS courses (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    quota INTEGER NOT NULL,
    enrolled_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT courses_quota_nonnegative CHECK (quota >= 0),
    CONSTRAINT courses_count_within_quota CHECK (enrolled_count >= 0 AND enrolled_count <= quota),
    CONSTRAINT courses_version_positive CHECK (version >= 1)
);

CREATE TABLE IF NOT EXISTS enrollments (
    id BIGSERIAL PRIMARY KEY,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enrollments_course_user_unique UNIQUE (course_id, user_id)
);

CREATE INDEX IF NOT EXISTS enrollments_user_idx ON enrollments (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS enrollments_course_user_unique
    ON enrollments (course_id, user_id);

CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(80) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    outbox_id BIGINT NOT NULL REFERENCES outbox(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL DEFAULT 'email',
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(16) NOT NULL DEFAULT 'sent',
    CONSTRAINT notifications_outbox_unique UNIQUE (outbox_id)
);

INSERT INTO permissions (name, description)
VALUES ('course:manage', 'Mengelola kelas dan kuotanya')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name)
VALUES ('admin', 'course:manage')
ON CONFLICT DO NOTHING;

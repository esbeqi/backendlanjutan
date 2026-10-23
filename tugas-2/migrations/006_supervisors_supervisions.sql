CREATE TABLE IF NOT EXISTS supervisors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    quota INTEGER NOT NULL,
    supervised_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT supervisors_quota_nonnegative
        CHECK (quota >= 0),
    CONSTRAINT supervisors_count_within_quota
        CHECK (supervised_count >= 0 AND supervised_count <= quota),
    CONSTRAINT supervisors_version_positive
        CHECK (version >= 1)
);

CREATE TABLE IF NOT EXISTS supervisions (
    student_id INTEGER NOT NULL,
    supervisor_id INTEGER NOT NULL,
    CONSTRAINT supervisions_student_id_unique UNIQUE (student_id),
    CONSTRAINT supervisions_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES students(id),
    CONSTRAINT supervisions_supervisor_id_fkey
        FOREIGN KEY (supervisor_id) REFERENCES supervisors(id)
);

CREATE INDEX IF NOT EXISTS supervisions_supervisor_id_idx
    ON supervisions (supervisor_id);

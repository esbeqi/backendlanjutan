ALTER TABLE students
ADD COLUMN IF NOT EXISTS owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS students_owner_id_idx
ON students (owner_id);

INSERT INTO permissions (name, description) VALUES
('student:list', 'Melihat daftar mahasiswa'),
('student:read:any', 'Melihat data mahasiswa lain'),
('student:create', 'Membuat data mahasiswa'),
('student:update:any', 'Mengubah data mahasiswa lain'),
('student:delete', 'Menghapus data mahasiswa')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name) VALUES
('admin', 'student:list'),
('admin', 'student:read:any'),
('admin', 'student:create'),
('admin', 'student:update:any'),
('admin', 'student:delete'),
('staff', 'student:list'),
('staff', 'student:read:any'),
('staff', 'student:create')
ON CONFLICT DO NOTHING;
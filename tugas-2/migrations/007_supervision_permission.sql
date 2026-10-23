INSERT INTO permissions (name, description)
VALUES ('supervision:manage', 'Menetapkan mahasiswa kepada supervisor')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name)
VALUES ('admin', 'supervision:manage')
ON CONFLICT DO NOTHING;

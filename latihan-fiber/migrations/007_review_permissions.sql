-- Permission moderasi review disimpan di PostgreSQL bersama permission RBAC lain.
INSERT INTO permissions (name, description)
VALUES ('review:moderate', 'Menghapus ulasan milik siapa pun')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name)
VALUES ('admin', 'review:moderate')
ON CONFLICT DO NOTHING;

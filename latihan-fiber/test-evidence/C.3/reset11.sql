-- reset11.sql - mengembalikan keadaan akun uji Modul 11
-- Hanya mengubah data milik dua akun uji yang ditentukan modul.

DELETE FROM notifications
WHERE user_id IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));

DELETE FROM outbox
WHERE (payload->>'user_id')::INT IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));

DELETE FROM enrollments
WHERE user_id IN (SELECT id FROM users WHERE username IN ('mhsuji', 'salahalamat'));

UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);

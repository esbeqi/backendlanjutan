INSERT INTO users (username, email, password, role)
SELECT
    'mhs' || LPAD(n::TEXT, 3, '0'),
    'mhs' || LPAD(n::TEXT, 3, '0') || '@student.unair.ac.id',
    '!',
    'user'
FROM GENERATE_SERIES(1, 60) AS series(n)
ON CONFLICT DO NOTHING;

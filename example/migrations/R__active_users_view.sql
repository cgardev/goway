DROP VIEW IF EXISTS active_users;

CREATE VIEW active_users AS
SELECT id, name, email
FROM users;

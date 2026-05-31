ALTER TABLE users ADD COLUMN email TEXT;

CREATE UNIQUE INDEX idx_users_email ON users (email);

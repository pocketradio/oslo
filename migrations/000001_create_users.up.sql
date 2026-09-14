CREATE TYPE user_role AS ENUM ('rider', 'driver');

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL CHECK (btrim(email) <> ''),
    password_hash TEXT NOT NULL CHECK (password_hash <> ''),
    role user_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

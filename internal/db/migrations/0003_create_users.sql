CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL   PRIMARY KEY,
    username   TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

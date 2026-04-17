CREATE TABLE IF NOT EXISTS pages (
    id             BIGSERIAL PRIMARY KEY,
    url            TEXT NOT NULL UNIQUE,
    language       TEXT NOT NULL,
    title          TEXT NOT NULL,
    summary        TEXT NOT NULL,
    content        TEXT NOT NULL,
    date_published TIMESTAMPTZ,
    date_modified  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS raw_pages (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT NOT NULL UNIQUE,
    raw_html        TEXT NOT NULL,
    last_scraped_at TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS questions (
    id         BIGSERIAL PRIMARY KEY,
    page_id    BIGINT NOT NULL REFERENCES raw_pages(id),
    text       VARCHAR NOT NULL,
    choices    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

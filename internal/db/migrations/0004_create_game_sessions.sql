DO $$ BEGIN
    CREATE TYPE game_mode AS ENUM ('solo', 'multiplayer');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE game_status AS ENUM ('waiting', 'active', 'finished');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS game_sessions (
    id                   BIGSERIAL    PRIMARY KEY,
    mode                 game_mode    NOT NULL,
    status               game_status  NOT NULL DEFAULT 'waiting',
    language             TEXT         NOT NULL,
    page_id              BIGINT       REFERENCES pages(id),
    current_question_id  BIGINT       REFERENCES questions(id),
    current_turn_user_id BIGINT       REFERENCES users(id),
    question_served_at   TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at          TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS game_participants (
    id         BIGSERIAL   PRIMARY KEY,
    session_id BIGINT      NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users(id),
    score      INTEGER     NOT NULL DEFAULT 0,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, user_id)
);

CREATE INDEX IF NOT EXISTS game_sessions_status_lang ON game_sessions (status, language);
CREATE INDEX IF NOT EXISTS game_participants_session  ON game_participants (session_id);

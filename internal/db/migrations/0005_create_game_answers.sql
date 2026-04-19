CREATE TABLE IF NOT EXISTS game_answers (
    id           BIGSERIAL   PRIMARY KEY,
    session_id   BIGINT      NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    user_id      BIGINT      NOT NULL REFERENCES users(id),
    question_id  BIGINT      NOT NULL REFERENCES questions(id),
    choice_index SMALLINT    NOT NULL,
    is_correct   BOOLEAN     NOT NULL,
    timed_out    BOOLEAN     NOT NULL DEFAULT FALSE,
    answered_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS game_answers_session_idx ON game_answers (session_id);

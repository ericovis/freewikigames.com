# GAME Package

The `internal/game` package implements all trivia game business logic. It calls DAOs in `internal/db` but never owns SQL.

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `Service` | `game.go` | Entry point; holds `*db.DB` and `timeout` (30s) |
| `SessionState` | `state.go` | Read-model returned to API handlers (no correct flags) |
| `QuestionView` | `state.go` | Question presented to client (text + choices text only) |
| `AnswerResult` | `question.go` | Result of submitting an answer |

## Session Lifecycle

```
solo:        [created] → active (immediately)
multiplayer: [created] → waiting → active (host calls StartSession)
             both:      active → finished (no questions remain)
```

## Responsibilities

- **session.go**: `CreateSolo`, `CreateMultiplayer`, `JoinSession`, `StartSession`
- **question.go**: `NextQuestion` (samples + records served_at), `AnswerQuestion` (timer + correctness + turn rotation)
- **state.go**: `GetState` (assembles `SessionState` for REST responses)
- **errors.go**: sentinel errors mapped to HTTP status codes by handlers

## Timing

`AnswerQuestion` computes `time.Since(session.QuestionServedAt) > 30s` server-side. Timed-out answers are recorded with `timed_out=true` and do not earn points. The turn still advances.

## Turn Rotation (Multiplayer)

After each answer the turn rotates to the next participant in `joined_at ASC` order (wrapping around). `SetTurn` on `GameSessionDAO` clears the current question and updates `current_turn_user_id` atomically.

## Testing

- Tests live in `internal/game/` with `package game`.
- Use `TestMain` + `truncateTables` pattern (see `internal/db/testhelper_test.go`) for DB-backed tests.
- No mocks — the game service is tested against a real PostgreSQL instance.

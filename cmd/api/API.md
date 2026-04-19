# API Reference — freewikigames.com

The game API is a JSON REST service (`cmd/api`) with a Server-Sent Events (SSE) endpoint for real-time multiplayer broadcasts. All state mutations go through REST; SSE is read-only from the client's perspective.

## Architecture Overview

```
cmd/api/
  main.go
  handler/        — one file per resource group (auth, session, gameplay, sse)
  middleware/     — JWT auth, logger, panic recovery
  router/         — wires all routes into *http.ServeMux
  sse/            — Hub (session → subscriber channels map) + Event formatting
```

Internal packages consumed:
- `internal/db` — DAOs (users, game_sessions, game_participants, game_answers, questions, pages)
- `internal/game` — business logic (session creation, question sampling, answer validation, turn rotation)

## Environment Variables

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `DATABASE_URL` | Yes | — | pgx DSN |
| `JWT_SECRET` | Yes | — | HMAC-SHA256 key; server exits if unset |
| `PORT` | No | `8080` | HTTP listen port |

## Authentication

Users have a username only — no password, no email. The JWT is the credential.

- Algorithm: **HS256**
- Claims: `sub` (user_id as string), `username`, `iat`, `exp` (24 hours from issue)
- Header: `Authorization: Bearer <token>`

---

## REST Endpoints

Base path: `/api/v1`. All request and response bodies are JSON.

### Auth

**No authentication required.**

#### `POST /api/v1/auth/register`

Creates a new user. Returns 409 if the username is already taken.

Request:
```json
{ "username": "alice" }
```

Response `200`:
```json
{ "token": "<jwt>", "user_id": 1, "username": "alice" }
```

#### `POST /api/v1/auth/login`

Looks up an existing user by username. Returns 401 if not found.

Request:
```json
{ "username": "alice" }
```

Response `200`:
```json
{ "token": "<jwt>", "user_id": 1, "username": "alice" }
```

---

### Sessions

**All endpoints require authentication.**

#### `POST /api/v1/sessions`

Creates a new game session. For **solo** mode the session immediately enters `active` state. For **multiplayer** it enters `waiting` state until the host calls `/start`.

Request:
```json
{
  "mode": "solo",
  "language": "en",
  "page_id": 42
}
```

- `mode`: `"solo"` or `"multiplayer"`
- `language`: must match a value present in the `pages.language` column (e.g. `"en"`, `"pt"`)
- `page_id`: optional. Omit to draw random questions from any page in that language.

Response `201`: `SessionState` (see [Common Types](#common-types))

#### `GET /api/v1/sessions?language=en&mode=multiplayer`

Lists open multiplayer sessions in `waiting` state for a given language. Useful for a lobby browser.

Query params:
- `language` (required)
- `mode` (optional, defaults to `multiplayer`)

Response `200`:
```json
{
  "sessions": [ ...SessionState ]
}
```

#### `GET /api/v1/sessions/{id}`

Returns current state of a session.

Response `200`: `SessionState`

#### `POST /api/v1/sessions/{id}/join`

Joins a multiplayer session that is still in `waiting` state. Returns 409 if already a participant or if the session has started.

Response `200`: `SessionState`

#### `POST /api/v1/sessions/{id}/start`

Starts a multiplayer session. Only the user who created the session (first participant) may call this. Returns 403 otherwise.

Response `200`: `SessionState`

---

### Gameplay

**All endpoints require authentication.**

#### `POST /api/v1/sessions/{id}/next`

Advances to the next question for the caller (solo) or for the current turn holder (multiplayer). Returns 409 if it is not the caller's turn. Returns 422 if no unanswered questions remain.

If the current question is still active (30-second window not expired), this is idempotent and returns the same question with an updated `seconds_left`.

Response `200`:
```json
{
  "question_id": 7,
  "text": "What year did X happen?",
  "choices": ["1914", "1939", "1945", "1066", "1776"],
  "seconds_left": 30
}
```

`choices` contains text only — the correct flag is never sent to the client before answering.

#### `POST /api/v1/sessions/{id}/answer`

Submits an answer for the current question. The `question_id` field must match the session's active question (prevents stale answers after turn changes).

Returns 422 if the answer arrives after the 30-second window. The server uses `NOW() - question_served_at` and never trusts client timestamps.

Request:
```json
{ "question_id": 7, "choice_index": 2 }
```

`choice_index`: 0-based index into the `choices` array returned by `/next`.

Response `200`:
```json
{
  "is_correct": true,
  "timed_out": false,
  "correct_index": 2,
  "score": 3
}
```

- `correct_index`: revealed after answering regardless of correctness
- `score`: caller's cumulative score in this session after this answer

---

## Server-Sent Events (SSE)

**Endpoint:** `GET /api/v1/sessions/{id}/events`

Auth: standard `Authorization: Bearer <token>` header.

Used for **multiplayer** sessions only. Solo mode has no real-time requirements; clients poll REST endpoints instead.

The connection is **server-to-client only**. Clients do not send messages — all game actions (join, start, next, answer) go through REST endpoints. The browser's native `EventSource` API handles automatic reconnection.

Response headers:
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

### Event Format

Each event follows the SSE wire format:

```
event: <type>
data: <json>

```

### Event Types

#### `session.started`
Fired when the host calls `POST /start`.
```
event: session.started
data: {"session_id":1,"status":"active","current_turn_user_id":5}
```

#### `participant.joined`
Fired when a user calls `POST /join`.
```
event: participant.joined
data: {"session_id":1,"user_id":8,"username":"bob","participant_count":2}
```

#### `question.served`
Fired when the current turn holder calls `POST /next`.
```
event: question.served
data: {"session_id":1,"question_id":7,"text":"What year did X happen?","choices":["1914","1939","1945","1066","1776"],"seconds_left":30,"current_turn_user_id":5}
```

#### `answer.result`
Fired after any participant calls `POST /answer`.
```
event: answer.result
data: {"session_id":1,"user_id":5,"question_id":7,"is_correct":true,"timed_out":false,"correct_index":2,"scores":{"5":3,"8":1}}
```

`scores` is `map[string]int` keyed by user_id-as-string.

#### `turn.changed`
Fired after each answer when the turn advances.
```
event: turn.changed
data: {"session_id":1,"current_turn_user_id":8}
```

#### `session.finished`
Fired when the session ends (no more questions).
```
event: session.finished
data: {"session_id":1,"status":"finished","scores":{"5":3,"8":1},"winner_user_id":5}
```

`winner_user_id` is the participant with the highest score. Ties are broken by join order.

---

## Common Types

### `SessionState`

Returned by `POST /sessions`, `GET /sessions/{id}`, `POST /join`, `POST /start`.

```json
{
  "session_id": 1,
  "mode": "multiplayer",
  "status": "active",
  "language": "en",
  "current_question": {
    "question_id": 7,
    "text": "What year did X happen?",
    "choices": ["1914", "1939", "1945", "1066", "1776"]
  },
  "participants": [
    { "user_id": 5, "username": "alice", "score": 3 },
    { "user_id": 8, "username": "bob",   "score": 1 }
  ],
  "current_turn_user_id": 8,
  "seconds_left": 22
}
```

- `current_question`: omitted when no question is active
- `current_turn_user_id`: omitted for solo sessions
- `seconds_left`: omitted when no question is active; computed as `30 - elapsed`

---

## Error Responses

All errors return JSON:

```json
{ "error": "human-readable message" }
```

| HTTP Status | Condition |
|-------------|-----------|
| 400 | Malformed request body or missing required field |
| 401 | Missing or invalid JWT; username not found on login |
| 403 | Action not permitted for the caller (e.g. non-host starting game) |
| 404 | Session or resource not found |
| 409 | Conflict — not your turn, already answered, already a participant |
| 422 | Answer submitted after 30-second window; no questions remaining |
| 500 | Unexpected server error |

---

## Game Rules

- **Language lock**: every session is bound to a language at creation time. Questions are always drawn from pages whose `language` matches the session language.
- **30-second timer**: the server records a `question_served_at` timestamp when `/next` is called. Answers arriving after 30 seconds are recorded as `timed_out` and do not earn points.
- **Solo**: one player, no SSE needed. The player calls `/next` to get a question and `/answer` to submit. The session ends when no unanswered questions remain.
- **Multiplayer**: turn-based. Only the current turn holder may call `/next`. After an answer (or timeout), the turn rotates to the next participant in join order. Results are broadcast to all SSE subscribers.
- **Scoring**: 1 point per correct, non-timed-out answer.

---

## Dependencies

```
github.com/golang-jwt/jwt/v5 v5.2.1
```

SSE is implemented with pure `net/http` stdlib — no additional library required.

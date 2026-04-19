package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GameSession represents a row in the game_sessions table.
type GameSession struct {
	ID                 int64
	Mode               string // "solo" | "multiplayer"
	Status             string // "waiting" | "active" | "finished"
	Language           string
	PageID             *int64
	CurrentQuestionID  *int64
	CurrentTurnUserID  *int64
	QuestionServedAt   *time.Time
	CreatedAt          time.Time
	FinishedAt         *time.Time
}

// GameParticipant represents a row in the game_participants table.
type GameParticipant struct {
	ID        int64
	SessionID int64
	UserID    int64
	Score     int
	JoinedAt  time.Time
}

// GameSessionDAO provides data-access methods for the game_sessions and
// game_participants tables.
type GameSessionDAO struct {
	pool *pgxpool.Pool
}

// Insert creates a new game session and returns the persisted row.
func (d *GameSessionDAO) Insert(ctx context.Context, mode, language string, pageID *int64) (*GameSession, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO game_sessions (mode, language, page_id)
		VALUES ($1, $2, $3)
		RETURNING id, mode, status, language, page_id,
		          current_question_id, current_turn_user_id, question_served_at,
		          created_at, finished_at
	`, mode, language, pageID)
	return scanSession(row)
}

// FindByID returns the session with the given id, or (nil, nil) if not found.
func (d *GameSessionDAO) FindByID(ctx context.Context, id int64) (*GameSession, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, mode, status, language, page_id,
		       current_question_id, current_turn_user_id, question_served_at,
		       created_at, finished_at
		FROM game_sessions
		WHERE id = $1
	`, id)
	s, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

// UpdateStatus sets the status of the given session.
func (d *GameSessionDAO) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE game_sessions SET status = $1 WHERE id = $2
	`, status, id)
	return err
}

// SetCurrentQuestion updates the current question, the turn holder, and the
// timestamp at which the question was served.
func (d *GameSessionDAO) SetCurrentQuestion(ctx context.Context, id, questionID int64, turnUserID *int64, servedAt time.Time) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE game_sessions
		SET current_question_id  = $1,
		    current_turn_user_id = $2,
		    question_served_at   = $3
		WHERE id = $4
	`, questionID, turnUserID, servedAt, id)
	return err
}

// SetTurn clears the current question and advances the turn to the given user.
// Called after an answer is submitted in multiplayer mode.
func (d *GameSessionDAO) SetTurn(ctx context.Context, sessionID, nextUserID int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE game_sessions
		SET current_question_id  = NULL,
		    question_served_at   = NULL,
		    current_turn_user_id = $1
		WHERE id = $2
	`, nextUserID, sessionID)
	return err
}

// SetFinished marks the session as finished and records the timestamp.
func (d *GameSessionDAO) SetFinished(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE game_sessions
		SET status      = 'finished',
		    finished_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

// ListWaiting returns up to limit multiplayer sessions in waiting state for the
// given language, ordered by created_at descending.
func (d *GameSessionDAO) ListWaiting(ctx context.Context, language string, limit int) ([]GameSession, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, mode, status, language, page_id,
		       current_question_id, current_turn_user_id, question_served_at,
		       created_at, finished_at
		FROM game_sessions
		WHERE status = 'waiting'
		  AND mode   = 'multiplayer'
		  AND language = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, language, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []GameSession
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

// InsertParticipant adds a user to a session and returns the persisted row.
func (d *GameSessionDAO) InsertParticipant(ctx context.Context, sessionID, userID int64) (*GameParticipant, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO game_participants (session_id, user_id)
		VALUES ($1, $2)
		RETURNING id, session_id, user_id, score, joined_at
	`, sessionID, userID)
	return scanParticipant(row)
}

// ListParticipants returns all participants for a session ordered by join time.
func (d *GameSessionDAO) ListParticipants(ctx context.Context, sessionID int64) ([]GameParticipant, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, session_id, user_id, score, joined_at
		FROM game_participants
		WHERE session_id = $1
		ORDER BY joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []GameParticipant
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		participants = append(participants, *p)
	}
	return participants, rows.Err()
}

// IncrementScore adds one point to the participant's score.
func (d *GameSessionDAO) IncrementScore(ctx context.Context, sessionID, userID int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE game_participants
		SET score = score + 1
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID)
	return err
}

func scanSession(row pgx.Row) (*GameSession, error) {
	var s GameSession
	err := row.Scan(
		&s.ID, &s.Mode, &s.Status, &s.Language, &s.PageID,
		&s.CurrentQuestionID, &s.CurrentTurnUserID, &s.QuestionServedAt,
		&s.CreatedAt, &s.FinishedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanParticipant(row pgx.Row) (*GameParticipant, error) {
	var p GameParticipant
	if err := row.Scan(&p.ID, &p.SessionID, &p.UserID, &p.Score, &p.JoinedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

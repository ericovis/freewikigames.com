package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GameAnswer represents a row in the game_answers table.
type GameAnswer struct {
	ID          int64
	SessionID   int64
	UserID      int64
	QuestionID  int64
	ChoiceIndex int
	IsCorrect   bool
	TimedOut    bool
	AnsweredAt  time.Time
}

// GameAnswerDAO provides data-access methods for the game_answers table.
type GameAnswerDAO struct {
	pool *pgxpool.Pool
}

// Insert records an answer submission.
func (d *GameAnswerDAO) Insert(ctx context.Context, sessionID, userID, questionID int64, choiceIndex int, isCorrect, timedOut bool) (*GameAnswer, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO game_answers (session_id, user_id, question_id, choice_index, is_correct, timed_out)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, session_id, user_id, question_id, choice_index, is_correct, timed_out, answered_at
	`, sessionID, userID, questionID, choiceIndex, isCorrect, timedOut)
	return scanAnswer(row)
}

// ListBySession returns all answers for a session ordered by answered_at ascending.
func (d *GameAnswerDAO) ListBySession(ctx context.Context, sessionID int64) ([]GameAnswer, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, session_id, user_id, question_id, choice_index, is_correct, timed_out, answered_at
		FROM game_answers
		WHERE session_id = $1
		ORDER BY answered_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []GameAnswer
	for rows.Next() {
		a, err := scanAnswer(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, *a)
	}
	return answers, rows.Err()
}

// ExistsForQuestion returns true if the user has already answered the given
// question in the given session.
func (d *GameAnswerDAO) ExistsForQuestion(ctx context.Context, sessionID, userID, questionID int64) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM game_answers
			WHERE session_id  = $1
			  AND user_id     = $2
			  AND question_id = $3
		)
	`, sessionID, userID, questionID).Scan(&exists)
	return exists, err
}

func scanAnswer(row pgx.Row) (*GameAnswer, error) {
	var a GameAnswer
	err := row.Scan(
		&a.ID, &a.SessionID, &a.UserID, &a.QuestionID,
		&a.ChoiceIndex, &a.IsCorrect, &a.TimedOut, &a.AnsweredAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

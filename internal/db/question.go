package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Choice represents a single answer option for a question.
type Choice struct {
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}

// Question represents a row in the questions table.
type Question struct {
	ID        int64
	PageID    int64
	Text      string
	Choices   []Choice
	CreatedAt time.Time
}

// QuestionDAO provides data-access methods for the questions table.
// Each method maps to a single SQL statement.
type QuestionDAO struct {
	pool *pgxpool.Pool
}

// Insert inserts a new question linked to the given page and returns the full
// persisted row.
func (d *QuestionDAO) Insert(ctx context.Context, pageID int64, text string, choices []Choice) (*Question, error) {
	choicesJSON, err := json.Marshal(choices)
	if err != nil {
		return nil, err
	}

	row := d.pool.QueryRow(ctx, `
		INSERT INTO questions (page_id, text, choices)
		VALUES ($1, $2, $3)
		RETURNING id, page_id, text, choices, created_at
	`, pageID, text, choicesJSON)

	return scanQuestion(row)
}

// FindByID returns the question with the given id, or (nil, nil) if not found.
func (d *QuestionDAO) FindByID(ctx context.Context, id int64) (*Question, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, page_id, text, choices, created_at
		FROM questions
		WHERE id = $1
	`, id)

	q, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return q, err
}

// FindByPage returns all questions for the given page ordered by id ascending.
func (d *QuestionDAO) FindByPage(ctx context.Context, pageID int64) ([]Question, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, page_id, text, choices, created_at
		FROM questions
		WHERE page_id = $1
		ORDER BY id ASC
	`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectQuestions(rows)
}

// Delete removes the question with the given id. It is a no-op if the id does
// not exist.
func (d *QuestionDAO) Delete(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM questions WHERE id = $1`, id)
	return err
}

// FindNextForSession returns a random question that has not yet been answered
// in the given session, filtered by language. If pageID is non-nil only
// questions from that page are considered. Returns (nil, nil) when no
// unanswered questions remain.
func (d *QuestionDAO) FindNextForSession(ctx context.Context, language string, pageID *int64, sessionID int64) (*Question, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT q.id, q.page_id, q.text, q.choices, q.created_at
		FROM   questions q
		JOIN   pages p ON p.id = q.page_id
		WHERE  p.language = $1
		  AND  ($2::bigint IS NULL OR q.page_id = $2)
		  AND  q.id NOT IN (
		           SELECT question_id FROM game_answers WHERE session_id = $3
		       )
		ORDER  BY RANDOM()
		LIMIT  1
	`, language, pageID, sessionID)

	q, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return q, err
}

// scanQuestion scans a single row into a Question, unmarshalling the JSONB
// choices column.
func scanQuestion(row pgx.Row) (*Question, error) {
	var q Question
	var choicesJSON []byte
	if err := row.Scan(&q.ID, &q.PageID, &q.Text, &choicesJSON, &q.CreatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(choicesJSON, &q.Choices); err != nil {
		return nil, err
	}
	return &q, nil
}

// collectQuestions drains a pgx.Rows cursor into a slice of Question.
func collectQuestions(rows pgx.Rows) ([]Question, error) {
	var questions []Question
	for rows.Next() {
		var q Question
		var choicesJSON []byte
		if err := rows.Scan(&q.ID, &q.PageID, &q.Text, &choicesJSON, &q.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(choicesJSON, &q.Choices); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

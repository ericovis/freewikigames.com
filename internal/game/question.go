package game

import (
	"context"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
)

// AnswerResult is returned after a player submits an answer.
type AnswerResult struct {
	IsCorrect  bool
	TimedOut   bool
	CorrectIdx int
	Score      int
}

// NextQuestion samples an unanswered question for the session and records the
// served-at timestamp. Returns the current question unchanged (with an updated
// seconds_left via GetState) if one is already active. Returns ErrNotYourTurn
// if it is not the caller's turn (multiplayer). Returns ErrNoQuestions when
// the question pool is exhausted.
func (s *Service) NextQuestion(ctx context.Context, sessionID, userID int64) (*db.Question, error) {
	session, err := s.db.GameSessions().FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrNotFound
	}
	if session.Status != "active" {
		return nil, ErrSessionNotActive
	}
	if session.Mode == "multiplayer" && (session.CurrentTurnUserID == nil || *session.CurrentTurnUserID != userID) {
		return nil, ErrNotYourTurn
	}

	// If a question is currently active and within the time window, return it.
	if session.CurrentQuestionID != nil && session.QuestionServedAt != nil {
		elapsed := time.Since(*session.QuestionServedAt)
		if elapsed <= s.timeout {
			return s.db.Questions().FindByID(ctx, *session.CurrentQuestionID)
		}
	}

	q, err := s.db.Questions().FindNextForSession(ctx, session.Language, session.PageID, sessionID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		// No more questions; finish the session.
		if err := s.db.GameSessions().SetFinished(ctx, sessionID); err != nil {
			return nil, err
		}
		return nil, ErrNoQuestions
	}

	var turnUID *int64
	if session.Mode == "multiplayer" {
		turnUID = &userID
	}

	now := time.Now().UTC()
	if err := s.db.GameSessions().SetCurrentQuestion(ctx, sessionID, q.ID, turnUID, now); err != nil {
		return nil, err
	}

	return q, nil
}

// AnswerQuestion validates and records the player's answer. It enforces the
// 30-second timer, deduplication, and turn ownership. On success it returns
// the result and advances the turn (multiplayer) or finishes the session when
// no more questions remain.
func (s *Service) AnswerQuestion(ctx context.Context, sessionID, userID, questionID int64, choiceIndex int) (*AnswerResult, error) {
	session, err := s.db.GameSessions().FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrNotFound
	}
	if session.Status != "active" {
		return nil, ErrSessionNotActive
	}
	if session.CurrentQuestionID == nil || *session.CurrentQuestionID != questionID {
		return nil, ErrNotFound
	}
	if session.Mode == "multiplayer" && (session.CurrentTurnUserID == nil || *session.CurrentTurnUserID != userID) {
		return nil, ErrNotYourTurn
	}

	already, err := s.db.GameAnswers().ExistsForQuestion(ctx, sessionID, userID, questionID)
	if err != nil {
		return nil, err
	}
	if already {
		return nil, ErrAlreadyAnswered
	}

	timedOut := session.QuestionServedAt != nil && time.Since(*session.QuestionServedAt) > s.timeout

	q, err := s.db.Questions().FindByID(ctx, questionID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrNotFound
	}

	correctIdx := correctIndex(q.Choices)
	isCorrect := !timedOut && choiceIndex == correctIdx

	if _, err := s.db.GameAnswers().Insert(ctx, sessionID, userID, questionID, choiceIndex, isCorrect, timedOut); err != nil {
		return nil, err
	}

	score := 0
	if isCorrect {
		if err := s.db.GameSessions().IncrementScore(ctx, sessionID, userID); err != nil {
			return nil, err
		}
	}

	// Compute updated score from DB (IncrementScore already committed).
	participants, err := s.db.GameSessions().ListParticipants(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, p := range participants {
		if p.UserID == userID {
			score = p.Score
			break
		}
	}

	// Advance turn for multiplayer.
	if session.Mode == "multiplayer" {
		if err := s.advanceTurn(ctx, sessionID, userID, participants); err != nil {
			return nil, err
		}
	}

	return &AnswerResult{
		IsCorrect:  isCorrect,
		TimedOut:   timedOut,
		CorrectIdx: correctIdx,
		Score:      score,
	}, nil
}

// advanceTurn rotates current_turn_user_id to the next participant in join order
// and clears the current question.
func (s *Service) advanceTurn(ctx context.Context, sessionID, currentUserID int64, participants []db.GameParticipant) error {
	if len(participants) == 0 {
		return nil
	}
	idx := 0
	for i, p := range participants {
		if p.UserID == currentUserID {
			idx = i
			break
		}
	}
	next := participants[(idx+1)%len(participants)]
	return s.db.GameSessions().SetTurn(ctx, sessionID, next.UserID)
}

// correctIndex returns the 0-based index of the correct choice.
func correctIndex(choices []db.Choice) int {
	for i, c := range choices {
		if c.Correct {
			return i
		}
	}
	return 0
}

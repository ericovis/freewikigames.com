package game

import (
	"context"
	"fmt"
	"time"
)

// QuestionView is the client-facing representation of a question.
// Correct flags are never included.
type QuestionView struct {
	QuestionID  int64    `json:"question_id"`
	Text        string   `json:"text"`
	Choices     []string `json:"choices"`
	SecondsLeft int      `json:"seconds_left"`
}

// ParticipantState is the client-facing representation of a session participant.
type ParticipantState struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Score    int    `json:"score"`
}

// SessionState is the read-model returned by API handlers.
type SessionState struct {
	SessionID       int64              `json:"session_id"`
	Mode            string             `json:"mode"`
	Status          string             `json:"status"`
	Language        string             `json:"language"`
	CurrentQuestion *QuestionView      `json:"current_question,omitempty"`
	Participants    []ParticipantState `json:"participants"`
	CurrentTurnUID  *int64             `json:"current_turn_user_id,omitempty"`
}

// GetState assembles a SessionState for the given session.
func (s *Service) GetState(ctx context.Context, sessionID int64) (*SessionState, error) {
	session, err := s.db.GameSessions().FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrNotFound
	}

	participants, err := s.db.GameSessions().ListParticipants(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Resolve usernames for all participants.
	states := make([]ParticipantState, 0, len(participants))
	for _, p := range participants {
		u, err := s.db.Users().FindByID(ctx, p.UserID)
		if err != nil {
			return nil, err
		}
		username := fmt.Sprintf("user_%d", p.UserID)
		if u != nil {
			username = u.Username
		}
		states = append(states, ParticipantState{
			UserID:   p.UserID,
			Username: username,
			Score:    p.Score,
		})
	}

	st := &SessionState{
		SessionID:      session.ID,
		Mode:           session.Mode,
		Status:         session.Status,
		Language:       session.Language,
		Participants:   states,
		CurrentTurnUID: session.CurrentTurnUserID,
	}

	// Attach current question view if a question is active.
	if session.CurrentQuestionID != nil && session.QuestionServedAt != nil {
		q, err := s.db.Questions().FindByID(ctx, *session.CurrentQuestionID)
		if err != nil {
			return nil, err
		}
		if q != nil {
			elapsed := time.Since(*session.QuestionServedAt)
			secondsLeft := int(s.timeout.Seconds()) - int(elapsed.Seconds())
			if secondsLeft < 0 {
				secondsLeft = 0
			}
			choices := make([]string, len(q.Choices))
			for i, c := range q.Choices {
				choices[i] = c.Text
			}
			st.CurrentQuestion = &QuestionView{
				QuestionID:  q.ID,
				Text:        q.Text,
				Choices:     choices,
				SecondsLeft: secondsLeft,
			}
		}
	}

	return st, nil
}

package game

import (
	"context"

	"github.com/ericovis/freewikigames.com/internal/db"
)

// CreateSoloParams holds the parameters for creating a solo session.
type CreateSoloParams struct {
	UserID   int64
	Language string
	PageID   *int64 // nil = random questions across the language
}

// CreateSolo creates a solo session in active state and adds the user as the
// sole participant.
func (s *Service) CreateSolo(ctx context.Context, p CreateSoloParams) (*db.GameSession, error) {
	session, err := s.db.GameSessions().Insert(ctx, "solo", p.Language, p.PageID)
	if err != nil {
		return nil, err
	}

	if err := s.db.GameSessions().UpdateStatus(ctx, session.ID, "active"); err != nil {
		return nil, err
	}
	session.Status = "active"

	if _, err := s.db.GameSessions().InsertParticipant(ctx, session.ID, p.UserID); err != nil {
		return nil, err
	}

	return session, nil
}

// CreateMultiplayerParams holds the parameters for creating a multiplayer session.
type CreateMultiplayerParams struct {
	HostUserID int64
	Language   string
	PageID     *int64
}

// CreateMultiplayer creates a multiplayer session in waiting state and adds
// the host as the first participant.
func (s *Service) CreateMultiplayer(ctx context.Context, p CreateMultiplayerParams) (*db.GameSession, error) {
	session, err := s.db.GameSessions().Insert(ctx, "multiplayer", p.Language, p.PageID)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.GameSessions().InsertParticipant(ctx, session.ID, p.HostUserID); err != nil {
		return nil, err
	}

	return session, nil
}

// JoinSession adds the user to an existing multiplayer session. Returns
// ErrNotFound if the session does not exist, ErrSessionNotWaiting if it has
// already started, and ErrForbidden if the user is already a participant.
func (s *Service) JoinSession(ctx context.Context, sessionID, userID int64) error {
	session, err := s.db.GameSessions().FindByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrNotFound
	}
	if session.Status != "waiting" {
		return ErrSessionNotWaiting
	}

	participants, err := s.db.GameSessions().ListParticipants(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, p := range participants {
		if p.UserID == userID {
			return ErrForbidden
		}
	}

	_, err = s.db.GameSessions().InsertParticipant(ctx, sessionID, userID)
	return err
}

// ListWaitingSessions returns open multiplayer lobbies for the given language.
func (s *Service) ListWaitingSessions(ctx context.Context, language string) ([]*SessionState, error) {
	sessions, err := s.db.GameSessions().ListWaiting(ctx, language, 50)
	if err != nil {
		return nil, err
	}
	states := make([]*SessionState, 0, len(sessions))
	for _, sess := range sessions {
		st, err := s.GetState(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		states = append(states, st)
	}
	return states, nil
}

// StartSession moves a multiplayer session from waiting to active. Only the
// host (first participant) may call this.
func (s *Service) StartSession(ctx context.Context, sessionID, requestingUserID int64) error {
	session, err := s.db.GameSessions().FindByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrNotFound
	}
	if session.Status != "waiting" {
		return ErrSessionNotWaiting
	}

	participants, err := s.db.GameSessions().ListParticipants(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(participants) == 0 || participants[0].UserID != requestingUserID {
		return ErrForbidden
	}

	return s.db.GameSessions().UpdateStatus(ctx, sessionID, "active")
}

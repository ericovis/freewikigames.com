package game

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("forbidden")
	ErrTimedOut          = errors.New("answer timed out")
	ErrAlreadyAnswered   = errors.New("question already answered")
	ErrNotYourTurn       = errors.New("not your turn")
	ErrSessionNotWaiting = errors.New("session is not in waiting state")
	ErrSessionNotActive  = errors.New("session is not active")
	ErrNoQuestions       = errors.New("no questions available")
)

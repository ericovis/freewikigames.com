package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ericovis/freewikigames.com/internal/game"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func httpStatusFor(err error) int {
	switch {
	case errors.Is(err, game.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, game.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, game.ErrNotYourTurn):
		return http.StatusConflict
	case errors.Is(err, game.ErrAlreadyAnswered):
		return http.StatusConflict
	case errors.Is(err, game.ErrSessionNotWaiting):
		return http.StatusConflict
	case errors.Is(err, game.ErrSessionNotActive):
		return http.StatusConflict
	case errors.Is(err, game.ErrTimedOut):
		return http.StatusUnprocessableEntity
	case errors.Is(err, game.ErrNoQuestions):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ericovis/freewikigames.com/cmd/api/middleware"
	"github.com/ericovis/freewikigames.com/internal/game"
)

// CreateSession handles POST /api/v1/sessions.
func CreateSession(svc *game.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Mode     string `json:"mode"`
			Language string `json:"language"`
			PageID   *int64 `json:"page_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Language == "" {
			writeError(w, http.StatusBadRequest, "language is required")
			return
		}

		switch body.Mode {
		case "solo":
			session, err := svc.CreateSolo(r.Context(), game.CreateSoloParams{
				UserID:   userID,
				Language: body.Language,
				PageID:   body.PageID,
			})
			if err != nil {
				writeError(w, httpStatusFor(err), err.Error())
				return
			}
			state, err := svc.GetState(r.Context(), session.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusCreated, state)

		case "multiplayer":
			session, err := svc.CreateMultiplayer(r.Context(), game.CreateMultiplayerParams{
				HostUserID: userID,
				Language:   body.Language,
				PageID:     body.PageID,
			})
			if err != nil {
				writeError(w, httpStatusFor(err), err.Error())
				return
			}
			state, err := svc.GetState(r.Context(), session.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusCreated, state)

		default:
			writeError(w, http.StatusBadRequest, "mode must be solo or multiplayer")
		}
	})
}

// GetSession handles GET /api/v1/sessions/{id}.
func GetSession(svc *game.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		state, err := svc.GetState(r.Context(), id)
		if err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, state)
	})
}

// ListSessions handles GET /api/v1/sessions?language=en.
func ListSessions(svc *game.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		language := r.URL.Query().Get("language")
		if language == "" {
			writeError(w, http.StatusBadRequest, "language query param is required")
			return
		}
		sessions, err := svc.ListWaitingSessions(r.Context(), language)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	})
}

// JoinSession handles POST /api/v1/sessions/{id}/join.
func JoinSession(svc *game.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		if err := svc.JoinSession(r.Context(), id, userID); err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}
		state, err := svc.GetState(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, state)
	})
}

// StartSession handles POST /api/v1/sessions/{id}/start.
func StartSession(svc *game.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		if err := svc.StartSession(r.Context(), id, userID); err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}
		state, err := svc.GetState(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, state)
	})
}

func parseIDParam(r *http.Request, param string) (int64, error) {
	return strconv.ParseInt(r.PathValue(param), 10, 64)
}

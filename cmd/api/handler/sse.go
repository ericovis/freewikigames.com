package handler

import (
	"fmt"
	"net/http"

	"github.com/ericovis/freewikigames.com/cmd/api/middleware"
	apisse "github.com/ericovis/freewikigames.com/cmd/api/sse"
	"github.com/ericovis/freewikigames.com/internal/game"
)

// Events handles GET /api/v1/sessions/{id}/events.
// It streams server-sent events to the caller for the duration of the
// connection. Authentication is via the standard Bearer header.
func Events(svc *game.Service, hub *apisse.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		sessionID, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		// Verify the session exists.
		if _, err := svc.GetState(r.Context(), sessionID); err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := hub.Subscribe(sessionID)
		defer hub.Unsubscribe(sessionID, ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case event, open := <-ch:
				if !open {
					return
				}
				if err := event.Write(w); err != nil {
					fmt.Fprintf(w, "event: error\ndata: {}\n\n")
					return
				}
				flusher.Flush()
			}
		}
	})
}

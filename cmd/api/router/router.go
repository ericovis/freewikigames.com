package router

import (
	"log/slog"
	"net/http"

	"github.com/ericovis/freewikigames.com/cmd/api/handler"
	"github.com/ericovis/freewikigames.com/cmd/api/middleware"
	"github.com/ericovis/freewikigames.com/cmd/api/sse"
	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/game"
)

// New builds and returns an http.Handler with all API routes registered.
func New(database *db.DB, svc *game.Service, hub *sse.Hub, jwtSecret string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	auth := middleware.Auth(jwtSecret)

	// Auth
	mux.Handle("POST /api/v1/auth/register", handler.Register(database.Users(), jwtSecret))
	mux.Handle("POST /api/v1/auth/login", handler.Login(database.Users(), jwtSecret))

	// Sessions
	mux.Handle("GET /api/v1/sessions", auth(handler.ListSessions(svc)))
	mux.Handle("POST /api/v1/sessions", auth(handler.CreateSession(svc)))
	mux.Handle("GET /api/v1/sessions/{id}", auth(handler.GetSession(svc)))
	mux.Handle("POST /api/v1/sessions/{id}/join", auth(handler.JoinSession(svc)))
	mux.Handle("POST /api/v1/sessions/{id}/start", auth(handler.StartSession(svc)))

	// Gameplay
	mux.Handle("POST /api/v1/sessions/{id}/next", auth(handler.NextQuestion(svc, hub)))
	mux.Handle("POST /api/v1/sessions/{id}/answer", auth(handler.AnswerQuestion(svc, hub)))

	// SSE
	mux.Handle("GET /api/v1/sessions/{id}/events", auth(handler.Events(svc, hub)))

	return middleware.Recover(logger)(middleware.Logger(logger)(mux))
}

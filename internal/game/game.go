package game

import (
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
)

const defaultTimeout = 30 * time.Second

// Service holds the game business logic. It calls DAOs but never owns SQL.
type Service struct {
	db      *db.DB
	timeout time.Duration
}

// NewService creates a new Service using the given database connection.
func NewService(database *db.DB) *Service {
	return &Service{db: database, timeout: defaultTimeout}
}

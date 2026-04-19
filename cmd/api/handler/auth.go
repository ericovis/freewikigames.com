package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

type authResponse struct {
	Token    string `json:"token"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

// Register creates a new user and returns a JWT. Returns 409 if the username
// is already taken.
func Register(users *db.UserDAO, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" {
			writeError(w, http.StatusBadRequest, "username is required")
			return
		}

		existing, err := users.FindByUsername(r.Context(), body.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if existing != nil {
			writeError(w, http.StatusConflict, "username already taken")
			return
		}

		user, err := users.Insert(r.Context(), body.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		token, err := issueJWT(user, secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token, UserID: user.ID, Username: user.Username})
	})
}

// Login looks up a user by username and returns a JWT. Returns 401 if not found.
func Login(users *db.UserDAO, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" {
			writeError(w, http.StatusBadRequest, "username is required")
			return
		}

		user, err := users.FindByUsername(r.Context(), body.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if user == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		token, err := issueJWT(user, secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token, UserID: user.ID, Username: user.Username})
	})
}

func issueJWT(user *db.User, secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      strconv.FormatInt(user.ID, 10),
		"username": user.Username,
		"iat":      now.Unix(),
		"exp":      now.Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

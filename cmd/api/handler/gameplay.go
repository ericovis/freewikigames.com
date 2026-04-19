package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ericovis/freewikigames.com/cmd/api/middleware"
	"github.com/ericovis/freewikigames.com/cmd/api/sse"
	"github.com/ericovis/freewikigames.com/internal/game"
)

type nextQuestionResponse struct {
	QuestionID  int64    `json:"question_id"`
	Text        string   `json:"text"`
	Choices     []string `json:"choices"`
	SecondsLeft int      `json:"seconds_left"`
}

type answerResponse struct {
	IsCorrect   bool `json:"is_correct"`
	TimedOut    bool `json:"timed_out"`
	CorrectIndex int  `json:"correct_index"`
	Score       int  `json:"score"`
}

// NextQuestion handles POST /api/v1/sessions/{id}/next.
func NextQuestion(svc *game.Service, hub *sse.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sessionID, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		q, err := svc.NextQuestion(r.Context(), sessionID, userID)
		if err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}

		choices := make([]string, len(q.Choices))
		for i, c := range q.Choices {
			choices[i] = c.Text
		}

		state, err := svc.GetState(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		secondsLeft := 30
		if state.CurrentQuestion != nil {
			secondsLeft = state.CurrentQuestion.SecondsLeft
		}

		resp := nextQuestionResponse{
			QuestionID:  q.ID,
			Text:        q.Text,
			Choices:     choices,
			SecondsLeft: secondsLeft,
		}

		// Broadcast to multiplayer subscribers.
		if state.Mode == "multiplayer" {
			hub.Broadcast(sessionID, sse.Event{
				Type: "question.served",
				Payload: map[string]any{
					"session_id":           sessionID,
					"question_id":          q.ID,
					"text":                 q.Text,
					"choices":              choices,
					"seconds_left":         secondsLeft,
					"current_turn_user_id": userID,
				},
			})
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// AnswerQuestion handles POST /api/v1/sessions/{id}/answer.
func AnswerQuestion(svc *game.Service, hub *sse.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromCtx(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sessionID, err := parseIDParam(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		var body struct {
			QuestionID  int64 `json:"question_id"`
			ChoiceIndex int   `json:"choice_index"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		result, err := svc.AnswerQuestion(r.Context(), sessionID, userID, body.QuestionID, body.ChoiceIndex)
		if err != nil {
			writeError(w, httpStatusFor(err), err.Error())
			return
		}

		// Broadcast results and state changes to multiplayer subscribers.
		state, stateErr := svc.GetState(r.Context(), sessionID)
		if stateErr == nil && state.Mode == "multiplayer" {
			scores := buildScores(state)
			hub.Broadcast(sessionID, sse.Event{
				Type: "answer.result",
				Payload: map[string]any{
					"session_id":    sessionID,
					"user_id":       userID,
					"question_id":   body.QuestionID,
					"is_correct":    result.IsCorrect,
					"timed_out":     result.TimedOut,
					"correct_index": result.CorrectIdx,
					"scores":        scores,
				},
			})
			if state.Status == "finished" {
				hub.Broadcast(sessionID, sse.Event{
					Type: "session.finished",
					Payload: map[string]any{
						"session_id":    sessionID,
						"status":        "finished",
						"scores":        scores,
						"winner_user_id": winnerUID(state),
					},
				})
			} else if state.CurrentTurnUID != nil {
				hub.Broadcast(sessionID, sse.Event{
					Type: "turn.changed",
					Payload: map[string]any{
						"session_id":           sessionID,
						"current_turn_user_id": *state.CurrentTurnUID,
					},
				})
			}
		}

		writeJSON(w, http.StatusOK, answerResponse{
			IsCorrect:    result.IsCorrect,
			TimedOut:     result.TimedOut,
			CorrectIndex: result.CorrectIdx,
			Score:        result.Score,
		})
	})
}

func buildScores(state *game.SessionState) map[string]int {
	scores := make(map[string]int, len(state.Participants))
	for _, p := range state.Participants {
		key := strconv.FormatInt(p.UserID, 10)
		scores[key] = p.Score
	}
	return scores
}

func winnerUID(state *game.SessionState) int64 {
	var winner int64
	var best int = -1
	for _, p := range state.Participants {
		if p.Score > best {
			best = p.Score
			winner = p.UserID
		}
	}
	return winner
}

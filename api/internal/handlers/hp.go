// Package handlers provides HTTP handlers for character state and actions.
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
)

// HPHandler handles HP-related HTTP requests.
type HPHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// GetHP returns the current HP state for the character.
func (h *HPHandler) GetHP(w http.ResponseWriter, _ *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := actor.GetHPState()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

// AppendEvent adds a new HP event (damage or healing) to the character.
func (h *HPHandler) AppendEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Delta  int    `json:"delta"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := actor.TakeDamage(body.Delta, body.Source); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := actor.GetHPState()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

// UndoLastEvent negates the effect of the most recent HP event.
func (h *HPHandler) UndoLastEvent(w http.ResponseWriter, _ *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := actor.UndoLastEvent(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := actor.GetHPState()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

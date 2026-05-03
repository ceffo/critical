package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// BuffsHandler provides HTTP handlers for managing character buffs.
type BuffsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// ListActive returns the list of currently active buffs for the character.
// ListActive returns the list of currently active buffs for the character.
func (h *BuffsHandler) ListActive(w http.ResponseWriter, _ *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, actor.ActiveBuffs)
}

// Activate adds a new buff to the character's active list.
func (h *BuffsHandler) Activate(w http.ResponseWriter, r *http.Request) {
	var buff domain.BuffEffect
	if err := json.NewDecoder(r.Body).Decode(&buff); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if buff.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := actor.ApplyBuff(buff); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, actor.ActiveBuffs)
}

// Dismiss removes an active buff from the character by its ID.
func (h *BuffsHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "id is required", http.StatusBadRequest)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := actor.DismissBuff(id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, actor.ActiveBuffs)
}

// Trigger decrements the charges of a charge-based buff.
func (h *BuffsHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "id is required", http.StatusBadRequest)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := actor.TriggerBuff(id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, actor.ActiveBuffs)
}

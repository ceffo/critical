package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type BuffsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

func (h *BuffsHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, actor.ActiveBuffs)
}

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

	buff.ID = uuid.New().String()

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

	newBuffs := []domain.BuffEffect{}
	for _, b := range actor.ActiveBuffs {
		if b.ID != id {
			newBuffs = append(newBuffs, b)
		}
	}

	actor.ActiveBuffs = newBuffs
	if err := actor.SaveActiveBuffs(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, actor.ActiveBuffs)
}

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

	actor.ActiveBuffs = domain.TriggerChargeBuff(actor.ActiveBuffs, id)

	if err := actor.SaveActiveBuffs(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, actor.ActiveBuffs)
}

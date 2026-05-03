package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/moncef-naji/critical/api/internal/domain"
)

type BuffsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

func (h *BuffsHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	buffs, err := domain.GetActiveBuffs(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, buffs)
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
	
	buffs, err := domain.GetActiveBuffs(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buffs = append(buffs, buff)
	if err := domain.SaveActiveBuffs(h.DB, h.CharacterID, buffs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, buffs)
}

func (h *BuffsHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "id is required", http.StatusBadRequest)
		return
	}

	buffs, err := domain.GetActiveBuffs(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newBuffs := []domain.BuffEffect{}
	for _, b := range buffs {
		if b.ID != id {
			newBuffs = append(newBuffs, b)
		}
	}

	if err := domain.SaveActiveBuffs(h.DB, h.CharacterID, newBuffs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, newBuffs)
}

func (h *BuffsHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "id is required", http.StatusBadRequest)
		return
	}

	buffs, err := domain.GetActiveBuffs(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newBuffs := domain.TriggerChargeBuff(buffs, id)

	if err := domain.SaveActiveBuffs(h.DB, h.CharacterID, newBuffs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, newBuffs)
}

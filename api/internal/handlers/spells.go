package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/moncef-naji/critical/api/internal/domain"
)

type SpellsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

func (h *SpellsHandler) GetSlots(w http.ResponseWriter, r *http.Request) {
	slots, err := domain.GetSpellSlots(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, slots)
}

func (h *SpellsHandler) Cast(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	index, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, "invalid slot index", http.StatusBadRequest)
		return
	}

	slots, err := domain.GetSpellSlots(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newSlots, err := domain.CastSpell(slots, index)
	if err != nil {
		if strings.Contains(err.Error(), "already used") {
			jsonError(w, err.Error(), http.StatusConflict)
		} else {
			jsonError(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	if err := domain.SaveSpellSlots(h.DB, h.CharacterID, newSlots); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, newSlots)
}

func (h *SpellsHandler) Reset(w http.ResponseWriter, r *http.Request) {
	slots, err := domain.GetSpellSlots(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newSlots := domain.ResetSpellSlots(slots)

	if err := domain.SaveSpellSlots(h.DB, h.CharacterID, newSlots); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, newSlots)
}

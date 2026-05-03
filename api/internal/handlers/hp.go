package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/moncef-naji/critical/api/internal/domain"
)

type HPHandler struct {
	DB          *sql.DB
	CharacterID int64
}

func (h *HPHandler) GetHP(w http.ResponseWriter, r *http.Request) {
	state, err := domain.GetHPState(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

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

	if err := domain.AppendEvent(h.DB, h.CharacterID, body.Delta, body.Source); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := domain.GetHPState(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

func (h *HPHandler) UndoLastEvent(w http.ResponseWriter, r *http.Request) {
	if err := domain.UndoLastEvent(h.DB, h.CharacterID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := domain.GetHPState(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, state)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

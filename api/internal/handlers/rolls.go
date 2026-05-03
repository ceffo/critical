package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/engine"
)

// RollsHandler handles generic dice rolling requests.
type RollsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// RollSave handles POST /api/roll/save requests.
func (h *RollsHandler) RollSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Category engine.SaveCategory `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dr := engine.NewDiceRoller()
	res := engine.ResolveSavingThrow(dr, req.Category, actor)

	jsonOK(w, res)
}

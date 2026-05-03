package handlers

import (
	"database/sql"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/moncef-naji/critical/api/internal/domain"
	"github.com/moncef-naji/critical/api/internal/engine"
)

type RollsHandler struct {
	DB          *sql.DB
	CharacterID int64
}

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

	dr := engine.NewDiceRoller(rand.NewSource(time.Now().UnixNano()))
	res := engine.ResolveSavingThrow(dr, req.Category, actor)

	jsonOK(w, res)
}

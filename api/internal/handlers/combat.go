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

type CombatHandler struct {
	DB          *sql.DB
	CharacterID int64
}

type AttackRequest struct {
	WeaponID string `json:"weapon_id"`
	TargetAC int    `json:"target_ac"`
}

var weapons = map[string]domain.Weapon{
	"longsword": {ID: "longsword", Name: "Longsword", BaseDamage: "1d8"},
	"shortbow":  {ID: "shortbow", Name: "Shortbow", BaseDamage: "1d6"},
}

func (h *CombatHandler) ResolveAttack(w http.ResponseWriter, r *http.Request) {
	var req AttackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	weapon, ok := weapons[req.WeaponID]
	if !ok {
		http.Error(w, "weapon not found", http.StatusNotFound)
		return
	}

	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dr := engine.NewDiceRoller(rand.NewSource(time.Now().UnixNano()))
	res := engine.ResolveAttack(dr, weapon, req.TargetAC, actor)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

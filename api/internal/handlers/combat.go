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

	// 1. Fetch character stats
	var char domain.Character
	err := h.DB.QueryRow(
		"SELECT id, name, str, thac0_base FROM characters WHERE id = ?",
		h.CharacterID,
	).Scan(&char.ID, &char.Name, &char.Str, &char.Thac0Base)
	if err != nil {
		http.Error(w, "failed to fetch character", http.StatusInternalServerError)
		return
	}

	// 2. Fetch active buffs
	var buffsJSON string
	err = h.DB.QueryRow(
		"SELECT buffs FROM active_buffs WHERE character_id = ?",
		h.CharacterID,
	).Scan(&buffsJSON)
	
	var activeBuffs []domain.BuffEffect
	if err == nil && buffsJSON != "" {
		json.Unmarshal([]byte(buffsJSON), &activeBuffs)
	} else if err != nil && err != sql.ErrNoRows {
		http.Error(w, "failed to fetch buffs", http.StatusInternalServerError)
		return
	}

	// 3. Resolve attack
	dr := engine.NewDiceRoller(rand.NewSource(time.Now().UnixNano()))
	res := engine.ResolveAttack(dr, weapon, req.TargetAC, char, activeBuffs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

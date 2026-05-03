// Package handlers provides the HTTP handlers for the critical API.
package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/engine"
)

// CombatHandler handles combat-related requests.
type CombatHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// AttackRequest represents a request to resolve an attack.
type AttackRequest struct {
	WeaponID string `json:"weapon_id"`
	TargetAC int    `json:"target_ac"`
}

var weapons = map[string]domain.Weapon{
	"longsword": {ID: "longsword", Name: "Longsword", BaseDamage: "1d8"},
	"shortbow":  {ID: "shortbow", Name: "Shortbow", BaseDamage: "1d6"},
}

// ResolveAttack handles POST /api/roll/attack requests.
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

	dr := engine.NewDiceRoller()
	res := engine.ResolveAttack(dr, weapon, req.TargetAC, actor)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

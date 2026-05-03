package handlers

import (
	"database/sql"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
)

// RoundHandler provides HTTP handlers for managing the game round and time-based effects.
type RoundHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// RoundAdvanceResponse encapsulates the new round number and the resulting buff state.
type RoundAdvanceResponse struct {
	RoundNumber  int                 `json:"roundNumber"`
	ActiveBuffs  []domain.BuffEffect `json:"activeBuffs"`
	ExpiredBuffs []domain.BuffEffect `json:"expiredBuffs"`
}

// GetRound returns the current round number.
func (h *RoundHandler) GetRound(w http.ResponseWriter, _ *http.Request) {
	round, err := domain.GetRoundNumber(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]int{"round": round})
}

// Advance increments the round number and processes buff expirations.
func (h *RoundHandler) Advance(w http.ResponseWriter, _ *http.Request) {
	// 1. Increment round counter
	round, err := domain.GetRoundNumber(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	round++
	if err := domain.SaveRoundNumber(h.DB, h.CharacterID, round); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Decrement duration buffs
	buffs, err := domain.GetActiveBuffs(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activeBuffs, expiredBuffs := domain.AdvanceRound(buffs)

	if err := domain.SaveActiveBuffs(h.DB, h.CharacterID, activeBuffs); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, RoundAdvanceResponse{
		RoundNumber:  round,
		ActiveBuffs:  activeBuffs,
		ExpiredBuffs: expiredBuffs,
	})
}

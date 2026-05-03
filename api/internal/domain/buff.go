package domain

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// BuffEffects represents the modifiers granted by a buff.
type BuffEffects struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
}

// BuffEffect represents a single active buff or debuff on a character.
type BuffEffect struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "charge" or "duration"
	Charges  int         `json:"charges"`
	Duration int         `json:"duration"`
	Effects  BuffEffects `json:"effects"`
}

// GetActiveBuffs retrieves all currently active buffs for a character.
func GetActiveBuffs(db *sql.DB, characterID int64) ([]BuffEffect, error) {
	var buffsJSON string
	err := db.QueryRow(
		"SELECT buffs FROM active_buffs WHERE character_id = ?",
		characterID,
	).Scan(&buffsJSON)
	if err == sql.ErrNoRows {
		return []BuffEffect{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active_buffs: %w", err)
	}

	var buffs []BuffEffect
	if buffsJSON != "" {
		if err := json.Unmarshal([]byte(buffsJSON), &buffs); err != nil {
			return nil, fmt.Errorf("unmarshal active_buffs: %w", err)
		}
	}
	return buffs, nil
}

// SaveActiveBuffs persists the list of active buffs for a character.
func SaveActiveBuffs(db *sql.DB, characterID int64, buffs []BuffEffect) error {
	buffsJSON, err := json.Marshal(buffs)
	if err != nil {
		return fmt.Errorf("marshal active_buffs: %w", err)
	}

	// Try to update first
	res, err := db.Exec(
		"UPDATE active_buffs SET buffs = ? WHERE character_id = ?",
		string(buffsJSON), characterID,
	)
	if err != nil {
		return fmt.Errorf("update active_buffs: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Insert if not exists
		_, err = db.Exec(
			"INSERT INTO active_buffs (character_id, buffs) VALUES (?, ?)",
			characterID, string(buffsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert active_buffs: %w", err)
		}
	}
	return nil
}

// AdvanceRound decrements duration-based buffs and returns the new active list and expired buffs.
func AdvanceRound(activeBuffs []BuffEffect) (newActive []BuffEffect, expired []BuffEffect) {
	for _, b := range activeBuffs {
		if b.Type == "duration" {
			b.Duration--
			if b.Duration > 0 {
				newActive = append(newActive, b)
			} else {
				expired = append(expired, b)
			}
		} else {
			newActive = append(newActive, b)
		}
	}
	return
}

// TriggerChargeBuff decrements a charge-based buff and returns the new active list.
func TriggerChargeBuff(activeBuffs []BuffEffect, id string) (newActive []BuffEffect) {
	for _, b := range activeBuffs {
		if b.ID == id && b.Type == "charge" {
			b.Charges--
			if b.Charges > 0 {
				newActive = append(newActive, b)
			}
		} else {
			newActive = append(newActive, b)
		}
	}
	return
}

// GetRoundNumber retrieves the current round number for a character from the database.
func GetRoundNumber(db *sql.DB, characterID int64) (int, error) {
	var round int
	err := db.QueryRow(
		"SELECT round_number FROM round_state WHERE character_id = ?",
		characterID,
	).Scan(&round)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query round_state: %w", err)
	}
	return round, nil
}

// SaveRoundNumber persists the current round number for a character to the database.
func SaveRoundNumber(db *sql.DB, characterID int64, round int) error {
	res, err := db.Exec(
		"UPDATE round_state SET round_number = ? WHERE character_id = ?",
		round, characterID,
	)
	if err != nil {
		return fmt.Errorf("update round_state: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		_, err = db.Exec(
			"INSERT INTO round_state (character_id, round_number) VALUES (?, ?)",
			characterID, round,
		)
		if err != nil {
			return fmt.Errorf("insert round_state: %w", err)
		}
	}
	return nil
}

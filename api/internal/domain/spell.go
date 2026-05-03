package domain

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Spell matches the schema in seeds/spells.json
type Spell struct {
	Name         string `json:"name"`
	Class        string `json:"class"`
	Level        int    `json:"level"`
	School       string `json:"school"`
	Sphere       string `json:"sphere"`
	CastingTime  string `json:"casting_time"`
	Range        string `json:"range"`
	Duration     string `json:"duration"`
	AreaOfEffect string `json:"area_of_effect"`
	Components   string `json:"components"`
	SavingThrow  string `json:"saving_throw"`
	Description  string `json:"description"`
}

// SpellSlot represents a prepared spell in a slot
type SpellSlot struct {
	Level             int    `json:"level"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
}

// GetSpellSlots retrieves the slots for a character from the database
func GetSpellSlots(db *sql.DB, characterID int64) ([]SpellSlot, error) {
	var slotsJSON string
	err := db.QueryRow("SELECT slots FROM spell_slots WHERE character_id = ?", characterID).Scan(&slotsJSON)
	if err == sql.ErrNoRows {
		return []SpellSlot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query spell_slots: %w", err)
	}

	var slots []SpellSlot
	if err := json.Unmarshal([]byte(slotsJSON), &slots); err != nil {
		return nil, fmt.Errorf("unmarshal spell_slots: %w", err)
	}
	return slots, nil
}

// SaveSpellSlots persists the slots for a character to the database
func SaveSpellSlots(db *sql.DB, characterID int64, slots []SpellSlot) error {
	slotsJSON, err := json.Marshal(slots)
	if err != nil {
		return fmt.Errorf("marshal spell_slots: %w", err)
	}

	res, err := db.Exec(
		"UPDATE spell_slots SET slots = ? WHERE character_id = ?",
		string(slotsJSON), characterID,
	)
	if err != nil {
		return fmt.Errorf("update spell_slots: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		_, err = db.Exec(
			"INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)",
			characterID, string(slotsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert spell_slots: %w", err)
		}
	}
	return nil
}

// CastSpell marks a specific slot as used
func CastSpell(slots []SpellSlot, index int) ([]SpellSlot, error) {
	if index < 0 || index >= len(slots) {
		return nil, fmt.Errorf("invalid slot index: %d", index)
	}
	if slots[index].Used {
		return nil, fmt.Errorf("slot %d already used", index)
	}

	newSlots := make([]SpellSlot, len(slots))
	copy(newSlots, slots)
	newSlots[index].Used = true
	return newSlots, nil
}

// ResetSpellSlots marks all slots as unused (long rest)
func ResetSpellSlots(slots []SpellSlot) []SpellSlot {
	newSlots := make([]SpellSlot, len(slots))
	for i, s := range slots {
		s.Used = false
		newSlots[i] = s
	}
	return newSlots
}

// SearchSpells searches for spells by name and class
func SearchSpells(db *sql.DB, query, class string) ([]Spell, error) {
	sqlQuery := "SELECT name, class, level, school, sphere, casting_time, range, duration, area_of_effect, components, saving_throw, description FROM spells WHERE 1=1"
	args := []interface{}{}

	if query != "" {
		sqlQuery += " AND name LIKE ?"
		args = append(args, "%"+query+"%")
	}
	if class != "" {
		sqlQuery += " AND class = ?"
		args = append(args, class)
	}

	sqlQuery += " ORDER BY level, name LIMIT 50"

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spells []Spell
	for rows.Next() {
		var s Spell
		err := rows.Scan(
			&s.Name, &s.Class, &s.Level, &s.School, &s.Sphere, &s.CastingTime, &s.Range,
			&s.Duration, &s.AreaOfEffect, &s.Components, &s.SavingThrow, &s.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("scan spell: %w", err)
		}
		spells = append(spells, s)
	}
	return spells, nil
}

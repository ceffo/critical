// Package domain contains the core business logic and state for characters, combat, and buffs.
package domain

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// Actor is a deep module encapsulating a character's stats, HP state, and active buffs.
type Actor struct {
	Stats       Character
	ActiveBuffs []BuffEffect
	db          *sql.DB
}

// LoadActor fetches the character and all related state from the DB
func LoadActor(db *sql.DB, id int64) (*Actor, error) {
	var char Character
	err := db.QueryRow(
		`SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
		        save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell
		 FROM characters WHERE id = ?`,
		id,
	).Scan(
		&char.ID, &char.Name, &char.Str, &char.Dex, &char.Con, &char.Int, &char.Wis, &char.Cha,
		&char.HPMax, &char.Thac0Base, &char.Level, &char.XPTotal,
		&char.SaveParaPoison, &char.SaveRodStaffWand, &char.SavePetriPoly, &char.SaveBreath, &char.SaveSpell,
	)
	if err != nil {
		return nil, fmt.Errorf("load character: %w", err)
	}

	buffs, err := GetActiveBuffs(db, id)
	if err != nil {
		return nil, fmt.Errorf("load active buffs: %w", err)
	}

	return &Actor{
		Stats:       char,
		ActiveBuffs: buffs,
		db:          db,
	}, nil
}

// ApplyBuff adds a new buff to the actor and saves to DB. Assigns a UUID if ID is empty.
func (a *Actor) ApplyBuff(buff BuffEffect) error {
	if buff.ID == "" {
		buff.ID = uuid.New().String()
	}
	a.ActiveBuffs = append(a.ActiveBuffs, buff)
	return SaveActiveBuffs(a.db, a.Stats.ID, a.ActiveBuffs)
}

// TakeDamage appends an HP event
func (a *Actor) TakeDamage(delta int, source string) error {
	return AppendEvent(a.db, a.Stats.ID, delta, source)
}

// UndoLastEvent negates the last HP event
func (a *Actor) UndoLastEvent() error {
	return UndoLastEvent(a.db, a.Stats.ID)
}

// GetHPState returns the current HP state
func (a *Actor) GetHPState() (*HPState, error) {
	return GetHPState(a.db, a.Stats.ID)
}

// TriggerBuff decrements charges on a charge-based buff and saves. Removes the buff when charges reach zero.
func (a *Actor) TriggerBuff(id string) error {
	a.ActiveBuffs = TriggerChargeBuff(a.ActiveBuffs, id)
	return SaveActiveBuffs(a.db, a.Stats.ID, a.ActiveBuffs)
}

// DismissBuff removes the buff with the given ID and saves. No-op if ID not found.
func (a *Actor) DismissBuff(id string) error {
	filtered := a.ActiveBuffs[:0]
	for _, b := range a.ActiveBuffs {
		if b.ID != id {
			filtered = append(filtered, b)
		}
	}
	a.ActiveBuffs = filtered
	return SaveActiveBuffs(a.db, a.Stats.ID, a.ActiveBuffs)
}

// SaveActiveBuffs persists the actor's buffs
func (a *Actor) SaveActiveBuffs() error {
	return SaveActiveBuffs(a.db, a.Stats.ID, a.ActiveBuffs)
}

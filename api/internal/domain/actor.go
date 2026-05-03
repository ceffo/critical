package domain

import (
	"database/sql"
	"fmt"
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
		"SELECT id, name, str, thac0_base FROM characters WHERE id = ?",
		id,
	).Scan(&char.ID, &char.Name, &char.Str, &char.Thac0Base)
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

// ApplyBuff adds a new buff to the actor and saves to DB
func (a *Actor) ApplyBuff(buff BuffEffect) error {
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

// SaveActiveBuffs persists the actor's buffs
func (a *Actor) SaveActiveBuffs() error {
	return SaveActiveBuffs(a.db, a.Stats.ID, a.ActiveBuffs)
}

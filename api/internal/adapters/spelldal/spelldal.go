// Package spelldal is the storage adapter for spellsvc. It satisfies both the
// write-side DAL and the read-side Reader on the same SQLite handle.
package spelldal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

type jsonSlot struct {
	Level             int    `json:"level"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
}

// GetSlots returns the slot list. An absent row is treated as an empty list.
func (d *DAL) GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error) {
	var blob string
	err := d.q.QueryRowContext(ctx, "SELECT slots FROM spell_slots WHERE character_id = ?", characterID).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return []spellsvc.SpellSlot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query spell_slots: %w", err)
	}
	if blob == "" {
		return []spellsvc.SpellSlot{}, nil
	}
	var raw []jsonSlot
	if err := json.Unmarshal([]byte(blob), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal spell_slots: %w", err)
	}
	out := make([]spellsvc.SpellSlot, 0, len(raw))
	for _, j := range raw {
		out = append(out, spellsvc.SpellSlot{
			Level: j.Level, PreparedSpellName: j.PreparedSpellName, Used: j.Used,
		})
	}
	return out, nil
}

// SaveSlots upserts the slot list.
func (d *DAL) SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error {
	raw := make([]jsonSlot, 0, len(slots))
	for _, s := range slots {
		raw = append(raw, jsonSlot{Level: s.Level, PreparedSpellName: s.PreparedSpellName, Used: s.Used})
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal spell_slots: %w", err)
	}
	res, err := d.q.ExecContext(ctx, "UPDATE spell_slots SET slots = ? WHERE character_id = ?", string(blob), characterID)
	if err != nil {
		return fmt.Errorf("update spell_slots: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		if _, err := d.q.ExecContext(ctx, "INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)", characterID, string(blob)); err != nil {
			return fmt.Errorf("insert spell_slots: %w", err)
		}
	}
	return nil
}

// Search runs the catalog query. Optional name fuzzy match and class filter.
func (d *DAL) Search(ctx context.Context, query, class string) ([]spellsvc.Spell, error) {
	q := "SELECT name, class, level, school, sphere, casting_time, range, duration, area_of_effect, components, saving_throw, description FROM spells WHERE 1=1"
	args := []any{}
	if query != "" {
		q += " AND name LIKE ?"
		args = append(args, "%"+query+"%")
	}
	if class != "" {
		q += " AND class = ?"
		args = append(args, class)
	}
	q += " ORDER BY level, name LIMIT 50"

	rows, err := d.q.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spells []spellsvc.Spell
	for rows.Next() {
		var s spellsvc.Spell
		if err := rows.Scan(
			&s.Name, &s.Class, &s.Level, &s.School, &s.Sphere, &s.CastingTime, &s.Range,
			&s.Duration, &s.AreaOfEffect, &s.Components, &s.SavingThrow, &s.Description,
		); err != nil {
			return nil, fmt.Errorf("scan spell: %w", err)
		}
		spells = append(spells, s)
	}
	return spells, rows.Err()
}

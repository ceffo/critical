// Package spelldal is the storage adapter for spellsvc. It delegates to
// bob-generated typed queries for slot operations and uses the bob query
// builder for the dynamic spell catalog search.
package spelldal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/spelldal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	bobmodels "github.com/ceffo/critical/api/internal/storage/sqlite/bob/models"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/sqlite/dialect"
	"github.com/stephenafamo/bob/dialect/sqlite/sm"
)

// DAL is the SQLite-backed implementation.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

// exec joins the transaction carried by ctx (sqlite.InTx), if any.
func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

type jsonSlot struct {
	Level             int    `json:"level"`
	PreparedSpellID   string `json:"prepared_spell_id,omitempty"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
}

// GetSlots returns the slot list. An absent row is treated as an empty list.
func (d *DAL) GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error) {
	blob, err := queries.GetSpellSlots(characterID).One(ctx, d.exec(ctx))
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
			Level: j.Level, SpellID: j.PreparedSpellID, PreparedSpellName: j.PreparedSpellName, Used: j.Used,
		})
	}
	return out, nil
}

// SaveSlots upserts the slot list.
func (d *DAL) SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error {
	raw := make([]jsonSlot, 0, len(slots))
	for _, s := range slots {
		raw = append(raw, jsonSlot{Level: s.Level, PreparedSpellID: s.SpellID, PreparedSpellName: s.PreparedSpellName, Used: s.Used})
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal spell_slots: %w", err)
	}
	_, err = queries.UpsertSpellSlots([]queries.UpsertSpellSlots_SpellSlots{
		{CharacterID: characterID, Slots: string(blob)},
	}).Exec(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("upsert spell_slots: %w", err)
	}
	return nil
}

// Search runs the catalog query. Optional name fuzzy match and class filter.
// Uses bob's model query builder because the WHERE conditions are dynamic.
func (d *DAL) Search(ctx context.Context, query, class string) ([]spellsvc.Spell, error) {
	where := bobmodels.SelectWhere.Spells
	mods := make([]bob.Mod[*dialect.SelectQuery], 0, 4)
	if query != "" {
		mods = append(mods, where.Name.Like("%"+query+"%"))
	}
	if class != "" {
		mods = append(mods, where.Class.EQ(class))
	}
	mods = append(mods,
		sm.OrderBy(bobmodels.Spells.Columns.Level),
		sm.OrderBy(bobmodels.Spells.Columns.Name),
		sm.Limit(50),
	)

	rows, err := bobmodels.Spells.Query(mods...).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	spells := make([]spellsvc.Spell, 0, len(rows))
	for _, r := range rows {
		spells = append(spells, spellsvc.Spell{
			Name: r.Name, Class: r.Class, Level: int(r.Level),
			School:       r.School.GetOrZero(),
			Sphere:       r.Sphere.GetOrZero(),
			CastingTime:  r.CastingTime.GetOrZero(),
			Range:        r.Range.GetOrZero(),
			Duration:     r.Duration.GetOrZero(),
			AreaOfEffect: r.AreaOfEffect.GetOrZero(),
			Components:   r.Components.GetOrZero(),
			SavingThrow:  r.SavingThrow.GetOrZero(),
			Description:  r.Description.GetOrZero(),
		})
	}
	return spells, nil
}

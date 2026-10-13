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
	"github.com/ceffo/critical/api/internal/domain/spellstats"
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

// Search runs the catalog query on the stored columns of f (name, class,
// level, school); the service applies the typed-stat filters.
// Uses bob's model query builder because the WHERE conditions are dynamic.
func (d *DAL) Search(ctx context.Context, f spellsvc.Filter) ([]spellsvc.Spell, error) {
	where := bobmodels.SelectWhere.Spells
	mods := make([]bob.Mod[*dialect.SelectQuery], 0, 4)
	if f.Query != "" {
		mods = append(mods, where.Name.Like("%"+f.Query+"%"))
	}
	if f.Class != "" {
		mods = append(mods, where.Class.Like(f.Class)) // LIKE without wildcards: case-insensitive equality
	}
	if f.Level != nil {
		mods = append(mods, where.Level.EQ(int64(*f.Level)))
	}
	if f.School != "" {
		mods = append(mods, where.School.Like("%"+f.School+"%"))
	}
	mods = append(mods,
		sm.OrderBy(bobmodels.Spells.Columns.Level),
		sm.OrderBy(bobmodels.Spells.Columns.Name),
		sm.OrderBy(bobmodels.Spells.Columns.Class),
	)

	rows, err := bobmodels.Spells.Query(mods...).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	keys, err := d.catalogKeys(ctx)
	if err != nil {
		return nil, err
	}
	spells := make([]spellsvc.Spell, 0, len(rows))
	for _, r := range rows {
		sp := toSpell(r)
		sp.ID = keys[r.ID]
		spells = append(spells, sp)
	}
	return spells, nil
}

// catalogKeys maps every row id to the spell's unique key (spellsvc.Keys).
// The table holds under a thousand static rows, so it reads them all.
func (d *DAL) catalogKeys(ctx context.Context) (map[int64]string, error) {
	rows, err := bobmodels.Spells.Query(sm.OrderBy(bobmodels.Spells.Columns.ID)).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query spell keys: %w", err)
	}
	refs := make([]spellsvc.Ref, len(rows))
	for i, r := range rows {
		refs[i] = spellsvc.Ref{Name: r.Name, Class: r.Class, Level: int(r.Level)}
	}
	keys := spellsvc.Keys(refs)
	out := make(map[int64]string, len(rows))
	for i, r := range rows {
		out[r.ID] = keys[i]
	}
	return out, nil
}

// Describe finds the spell whose unique key (spellsvc.Keys) is spellID.
func (d *DAL) Describe(ctx context.Context, spellID string) (spellsvc.Spell, bool, error) {
	keys, err := d.catalogKeys(ctx)
	if err != nil {
		return spellsvc.Spell{}, false, err
	}
	rows, err := bobmodels.Spells.Query().All(ctx, d.exec(ctx))
	if err != nil {
		return spellsvc.Spell{}, false, fmt.Errorf("query spells: %w", err)
	}
	for _, r := range rows {
		if keys[r.ID] == spellID {
			sp := toSpell(r)
			sp.ID = spellID
			return sp, true, nil
		}
	}
	return spellsvc.Spell{}, false, nil
}

func toSpell(r *bobmodels.Spell) spellsvc.Spell {
	sp := toSpellText(r)
	// stats is filled at seed time (and backfilled for older databases); a row
	// without it, or with unreadable JSON, is parsed again from its raw text.
	if blob := r.Stats.GetOrZero(); blob == "" || json.Unmarshal([]byte(blob), &sp.Stats) != nil {
		sp.Stats = spellstats.Parse(spellstats.Raw{
			CastingTime: sp.CastingTime, Range: sp.Range, Duration: sp.Duration,
			AreaOfEffect: sp.AreaOfEffect, Components: sp.Components, SavingThrow: sp.SavingThrow,
		})
	}
	return sp
}

func toSpellText(r *bobmodels.Spell) spellsvc.Spell {
	return spellsvc.Spell{
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
	}
}

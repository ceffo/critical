// Package spelldal is the storage adapter for spellsvc. It delegates to
// bob-generated typed queries for slot operations and uses the bob query
// builder for the dynamic spell catalog search.
package spelldal

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/aarondl/opt/null"
	"github.com/stephenafamo/bob"
	sqlitebob "github.com/stephenafamo/bob/dialect/sqlite"
	"github.com/stephenafamo/bob/dialect/sqlite/dialect"

	"github.com/ceffo/critical/api/internal/adapters/spelldal/queries"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	bobmodels "github.com/ceffo/critical/api/internal/storage/sqlite/bob/models"
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
	Extra             bool   `json:"extra,omitempty"`
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
			Level: j.Level, SpellID: j.PreparedSpellID, PreparedSpellName: j.PreparedSpellName, Used: j.Used, Extra: j.Extra,
		})
	}
	return out, nil
}

// SaveSlots upserts the slot list.
func (d *DAL) SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error {
	raw := make([]jsonSlot, 0, len(slots))
	for _, s := range slots {
		raw = append(raw, jsonSlot{Level: s.Level, PreparedSpellID: s.SpellID, PreparedSpellName: s.PreparedSpellName, Used: s.Used, Extra: s.Extra})
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

// Search runs the catalog query on the stored columns of f (class, levels),
// ordered by level, fallback-language name, class; the service applies the
// rest of the filter (names, school groups, spheres, typed stats) and ranks
// the matches. Only the name, school and sphere texts are read (one batched
// lookup by key, ADR 0023): never the descriptions. Uses bob's model query
// builder because the WHERE conditions are dynamic.
func (d *DAL) Search(ctx context.Context, f spellsvc.Filter) ([]spellsvc.Spell, error) {
	where := bobmodels.SelectWhere.Spells
	// Rows without a key are older duplicates the catalogue import left in
	// place (ADR 0021 step 5.4.1): never served.
	mods := make([]bob.Mod[*dialect.SelectQuery], 0, 3)
	mods = append(mods, where.Key.IsNotNull())
	if f.Class != "" {
		mods = append(mods, where.Class.Like(f.Class)) // LIKE without wildcards: case-insensitive equality
	}
	if len(f.Levels) > 0 {
		levels := make([]int64, len(f.Levels))
		for i, l := range f.Levels {
			levels[i] = int64(l)
		}
		mods = append(mods, where.Level.In(levels...))
	}
	rows, err := bobmodels.Spells.Query(mods...).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	ids := make([]int64, 0, 3*len(rows))
	for _, r := range rows {
		ids = appendIDs(ids, r.Name, r.School, r.Sphere)
		if _, ok := storedStats(r); !ok {
			ids = appendIDs(ids, statColumns(r)...) // re-parsed from the raw texts
		}
	}
	texts, err := d.texts(ctx, ids)
	if err != nil {
		return nil, err
	}
	spells := make([]spellsvc.Spell, 0, len(rows))
	for _, r := range rows {
		sp := spellsvc.Spell{
			ID: r.Key.GetOrZero(), Class: r.Class, Level: int(r.Level), Source: r.Source.GetOrZero(),
			Name: texts.of(r.Name), School: texts.of(r.School), Sphere: texts.of(r.Sphere),
		}
		sp.Stats = stats(r, texts)
		spells = append(spells, sp)
	}
	slices.SortStableFunc(spells, func(a, b spellsvc.Spell) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.Name.Default(), b.Name.Default()),
			cmp.Compare(a.Class, b.Class))
	})
	return spells, nil
}

// Details returns the full spells of ids, in that order, skipping an id no
// served row holds: one query for the rows, one batched lookup for all their
// texts.
func (d *DAL) Details(ctx context.Context, ids []string) ([]spellsvc.Spell, error) {
	if len(ids) == 0 {
		return []spellsvc.Spell{}, nil
	}
	rows, err := bobmodels.Spells.Query(bobmodels.SelectWhere.Spells.Key.In(ids...)).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query spells: %w", err)
	}
	textIDs := make([]int64, 0, 10*len(rows))
	byKey := make(map[string]*bobmodels.Spell, len(rows))
	for _, r := range rows {
		byKey[r.Key.GetOrZero()] = r
		textIDs = appendIDs(textIDs, append(statColumns(r), r.Name, r.School, r.Sphere, r.Description)...)
	}
	texts, err := d.texts(ctx, textIDs)
	if err != nil {
		return nil, err
	}
	out := make([]spellsvc.Spell, 0, len(ids))
	for _, id := range ids {
		if r, ok := byKey[id]; ok {
			out = append(out, toSpell(r, texts))
		}
	}
	return out, nil
}

// Describe finds the spell whose catalogue key is spellID.
func (d *DAL) Describe(ctx context.Context, spellID string) (spellsvc.Spell, bool, error) {
	found, err := d.Details(ctx, []string{spellID})
	if err != nil {
		return spellsvc.Spell{}, false, fmt.Errorf("query spell: %w", err)
	}
	if len(found) == 0 {
		return spellsvc.Spell{}, false, nil
	}
	return found[0], true, nil
}

// textBatch bounds the keys of one lookup, below SQLite's limit on bound
// parameters.
const textBatch = 5000

// textsByID are localized texts by key.
type textsByID map[int64]localized.Text

// of is the text a column keys (empty for NULL).
func (t textsByID) of(id null.Val[int64]) localized.Text {
	if v, ok := id.Get(); ok {
		return t[v]
	}
	return nil
}

// texts reads the localized_texts rows of ids through the primary key
// (text_id, lang), in batches of textBatch keys.
func (d *DAL) texts(ctx context.Context, ids []int64) (textsByID, error) {
	out := make(textsByID, len(ids))
	for start := 0; start < len(ids); start += textBatch {
		chunk := ids[start:min(start+textBatch, len(ids))]
		rows, err := TextsQuery(chunk).All(ctx, d.exec(ctx))
		if err != nil {
			return nil, fmt.Errorf("query localized texts: %w", err)
		}
		for _, r := range rows {
			if out[r.TextID] == nil {
				out[r.TextID] = localized.Text{}
			}
			out[r.TextID][r.Lang] = r.Body
		}
	}
	return out, nil
}

// TextsQuery is the lookup of the localized_texts rows of ids (exported for
// the query-plan test: it must search the primary key, never scan the table).
func TextsQuery(ids []int64) *sqlitebob.ViewQuery[*bobmodels.LocalizedText, bobmodels.LocalizedTextSlice] {
	return bobmodels.LocalizedTexts.Query(bobmodels.SelectWhere.LocalizedTexts.TextID.In(ids...))
}

// appendIDs appends the keys of the non-NULL columns.
func appendIDs(ids []int64, cols ...null.Val[int64]) []int64 {
	for _, c := range cols {
		if v, ok := c.Get(); ok {
			ids = append(ids, v)
		}
	}
	return ids
}

// statColumns are the keys of the six stat texts.
func statColumns(r *bobmodels.Spell) []null.Val[int64] {
	return []null.Val[int64]{r.CastingTime, r.Range, r.Duration, r.AreaOfEffect, r.Components, r.SavingThrow}
}

// storedStats decodes the stats filled at import.
func storedStats(r *bobmodels.Spell) (spellstats.Stats, bool) {
	var st spellstats.Stats
	blob := r.Stats.GetOrZero()
	return st, blob != "" && json.Unmarshal([]byte(blob), &st) == nil
}

// stats is the stored typed stats; a row without them, or with unreadable
// JSON, is parsed again from its fallback-language stat texts (texts must
// hold them).
func stats(r *bobmodels.Spell, texts textsByID) spellstats.Stats {
	if st, ok := storedStats(r); ok {
		return st
	}
	return spellstats.Parse(spellstats.Raw{
		CastingTime: texts.of(r.CastingTime).Default(), Range: texts.of(r.Range).Default(),
		Duration: texts.of(r.Duration).Default(), AreaOfEffect: texts.of(r.AreaOfEffect).Default(),
		Components: texts.of(r.Components).Default(), SavingThrow: texts.of(r.SavingThrow).Default(),
	})
}

func toSpell(r *bobmodels.Spell, texts textsByID) spellsvc.Spell {
	return spellsvc.Spell{
		ID: r.Key.GetOrZero(), Class: r.Class, Level: int(r.Level), Source: r.Source.GetOrZero(),
		Name:         texts.of(r.Name),
		School:       texts.of(r.School),
		Sphere:       texts.of(r.Sphere),
		CastingTime:  texts.of(r.CastingTime),
		Range:        texts.of(r.Range),
		Duration:     texts.of(r.Duration),
		AreaOfEffect: texts.of(r.AreaOfEffect),
		Components:   texts.of(r.Components),
		SavingThrow:  texts.of(r.SavingThrow),
		Description:  texts.of(r.Description),
		Stats:        stats(r, texts),
	}
}

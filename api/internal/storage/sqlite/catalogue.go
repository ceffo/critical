package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// ImportCatalogue imports the spell catalogue artifact into the spells table
// (ADR 0021 step 5, after the file was read and validated: steps 5.1 and 5.2
// are catalogueadpt's). seedRefs are the spell ids the seeds refer to (the
// spellbook and the spell effects); the ones game state holds are read from
// the database here.
//
//   - 5.3: the artifact version equals the recorded one: nothing is written.
//   - 5.4, in one transaction: 5.4.1 rows without a key are keyed (spellsvc.Keys
//     over the existing rows, the oldest row of a duplicated (name, class,
//     level) only) and each key is verified against the artifact; 5.4.2 every
//     artifact spell is upserted by key; 5.4.3 keyed spells the artifact no
//     longer holds are deleted, unless something refers to them, which rolls
//     the import back.
//   - 5.5: format and version are recorded in the same transaction.
//   - 5.6: the typed stats are parsed from the six raw texts on upsert.
func (d *DB) ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	same, err := sameVersion(ctx, d, cat)
	if err != nil || same {
		return err
	}
	return d.WrapTx(ctx, func(q Querier) error {
		if same, err := sameVersion(ctx, q, cat); err != nil || same {
			return err
		}
		if err := backfillKeys(ctx, q, cat); err != nil {
			return err
		}
		if err := upsertSpells(ctx, q, cat); err != nil {
			return err
		}
		if err := deleteRemoved(ctx, q, cat, seedRefs); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `
			INSERT INTO catalogue_meta (id, format, version) VALUES (1, ?, ?)
			ON CONFLICT (id) DO UPDATE SET format = excluded.format, version = excluded.version`,
			cat.Format, cat.Version); err != nil {
			return fmt.Errorf("record catalogue version: %w", err)
		}
		return nil
	})
}

// sameVersion: step 5.3, equality only (a rollback to an older artifact is an
// import like any other).
func sameVersion(ctx context.Context, q Querier, cat catalogue.Catalogue) (bool, error) {
	var format int
	var version string
	err := q.QueryRowContext(ctx, `SELECT format, version FROM catalogue_meta WHERE id = 1`).Scan(&format, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read catalogue version: %w", err)
	}
	return format == cat.Format && version == cat.Version, nil
}

type spellRef struct {
	id    int64
	key   sql.NullString
	name  string
	class string
	level int
}

func (r spellRef) triple() string {
	return fmt.Sprintf("(%q, %s, %d)", r.name, r.class, r.level)
}

// backfillKeys is step 5.4.1: the first import on a database seeded before
// the catalogue. Keys over every existing row reproduces the ids in use; the
// oldest row of a (name, class, level) takes its key, the others stay NULL
// (never served, never updated). Every key given is checked against the
// artifact spell holding it: a different (name, class, level) means the
// existing rows are not the set the ids were published from, and the import
// refuses rather than overwrite a different spell.
func backfillKeys(ctx context.Context, q Querier, cat catalogue.Catalogue) error {
	rows, err := q.QueryContext(ctx, `SELECT id, key, name, class, level FROM spells ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read spells: %w", err)
	}
	var all []spellRef
	for rows.Next() {
		var r spellRef
		if err := rows.Scan(&r.id, &r.key, &r.name, &r.class, &r.level); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan spell: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !slices.ContainsFunc(all, func(r spellRef) bool { return !r.key.Valid }) {
		return nil
	}

	refs := make([]spellsvc.Ref, len(all))
	taken := map[string]bool{}
	for i, r := range all {
		refs[i] = spellsvc.Ref{Name: r.name, Class: r.class, Level: r.level}
		if r.key.Valid {
			taken[r.key.String] = true
		}
	}
	keys := spellsvc.Keys(refs)
	artifact := cat.ByID()
	var mismatches []string
	type assignment struct {
		id  int64
		key string
	}
	var assign []assignment
	for i, r := range all {
		k := keys[i]
		if r.key.Valid || taken[k] {
			continue // keyed already, or a younger row of a duplicated triple
		}
		taken[k] = true
		if s, ok := artifact[k]; ok && (s.Name != r.name || s.Class != r.class || s.Level != r.level) {
			mismatches = append(mismatches, fmt.Sprintf("%s: database row %d is %s, the artifact has %s",
				k, r.id, r.triple(), spellRef{name: s.Name, class: s.Class, level: s.Level}.triple()))
			continue
		}
		assign = append(assign, assignment{r.id, k})
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("spell catalogue: the existing spells do not match the artifact's ids, refusing to import: %s",
			strings.Join(mismatches, "; "))
	}
	for _, a := range assign {
		if _, err := q.ExecContext(ctx, `UPDATE spells SET key = ? WHERE id = ?`, a.key, a.id); err != nil {
			return fmt.Errorf("key spell %d: %w", a.id, err)
		}
	}
	return nil
}

// upsertSpells is step 5.4.2 (with the typed stats of 5.6): insert a new key,
// overwrite every catalogue field of an existing one. An absent French text
// is stored as NULL.
func upsertSpells(ctx context.Context, q Querier, cat catalogue.Catalogue) error {
	for _, s := range cat.Spells {
		stats, err := spellStatsJSON(s.CastingTime, s.Range, s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow)
		if err != nil {
			return fmt.Errorf("spell %q: %w", s.ID, err)
		}
		if _, err := q.ExecContext(ctx, `
			INSERT INTO spells (
				key, name, name_fr, class, level, school, sphere, casting_time, range, duration,
				area_of_effect, components, saving_throw, description, description_fr, source, stats
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET
				name = excluded.name, name_fr = excluded.name_fr, class = excluded.class, level = excluded.level,
				school = excluded.school, sphere = excluded.sphere, casting_time = excluded.casting_time,
				range = excluded.range, duration = excluded.duration, area_of_effect = excluded.area_of_effect,
				components = excluded.components, saving_throw = excluded.saving_throw,
				description = excluded.description, description_fr = excluded.description_fr,
				source = excluded.source, stats = excluded.stats`,
			s.ID, s.Name, nullable(s.NameFR), s.Class, s.Level, s.School, nullable(s.Sphere), s.CastingTime, s.Range,
			s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow, s.Description, nullable(s.DescriptionFR),
			s.Source, stats,
		); err != nil {
			return fmt.Errorf("upsert spell %q: %w", s.ID, err)
		}
	}
	return nil
}

func nullable(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// deleteRemoved is step 5.4.3: a keyed spell the artifact no longer holds is
// deleted when nothing refers to it. Any reference fails the import with
// every such key and where it is used.
func deleteRemoved(ctx context.Context, q Querier, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	removed, err := removedKeys(ctx, q, cat)
	if err != nil || len(removed) == 0 {
		return err
	}
	stateRefs, err := StateSpellRefs(ctx, q)
	if err != nil {
		return err
	}
	uses := map[string][]string{}
	for _, r := range append(append([]catalogue.Ref(nil), seedRefs...), stateRefs...) {
		if removed[r.ID] && !slices.Contains(uses[r.ID], r.Where) {
			uses[r.ID] = append(uses[r.ID], r.Where)
		}
	}
	if len(uses) > 0 {
		keys := make([]string, 0, len(uses))
		for k := range uses {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		list := make([]string, len(keys))
		for i, k := range keys {
			list[i] = k + " (" + strings.Join(uses[k], ", ") + ")"
		}
		recorded, err := recordedCatalogue(ctx, q)
		if err != nil {
			return err
		}
		return fmt.Errorf("spell catalogue: the artifact (version %s) removes spells that are still referenced, refusing to import: %s. "+
			"This database was built from %s: start it with that catalogue (the real one is fetched by `just catalogue` "+
			"to .catalogue/catalogue.json, the default CRITICAL_CATALOGUE), or start this catalogue on a database of its own",
			cat.Version, strings.Join(list, "; "), recorded)
	}
	for k := range removed {
		if _, err := q.ExecContext(ctx, `DELETE FROM spells WHERE key = ?`, k); err != nil {
			return fmt.Errorf("delete spell %q: %w", k, err)
		}
	}
	return nil
}

// recordedCatalogue names the catalogue the database was last imported from,
// for the refusal message of deleteRemoved: a database used with the real
// catalogue started on the fixture (or the reverse) is refused there.
func recordedCatalogue(ctx context.Context, q Querier) (string, error) {
	var version string
	err := q.QueryRowContext(ctx, `SELECT version FROM catalogue_meta WHERE id = 1`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return "the spell seeds that predate the catalogue artifact (the real catalogue's spells)", nil
	}
	if err != nil {
		return "", fmt.Errorf("read catalogue version: %w", err)
	}
	return "catalogue version " + version, nil
}

func removedKeys(ctx context.Context, q Querier, cat catalogue.Catalogue) (map[string]bool, error) {
	artifact := cat.ByID()
	rows, err := q.QueryContext(ctx, `SELECT key FROM spells WHERE key IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("read spell keys: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan spell key: %w", err)
		}
		if _, ok := artifact[k]; !ok {
			out[k] = true
		}
	}
	return out, rows.Err()
}

// StateSpellRefs returns every spell id game state holds (ADR 0021 step
// 5.4.3): the prepared spells of spell_slots, the slot lists in the
// before/after state of action_snapshots (undo and redo restore them), the
// spellId of cast and exposure entries in action_log and of the payloads of
// their corrections, and spells[].id of spell_prep entries. A JSON value that
// does not decode is an error: an unreadable reference is not a missing one.
func StateSpellRefs(ctx context.Context, q Querier) ([]catalogue.Ref, error) {
	var out []catalogue.Ref
	add := func(id, where string) {
		if id != "" {
			out = append(out, catalogue.Ref{ID: id, Where: where})
		}
	}

	type slot struct {
		PreparedSpellID string `json:"prepared_spell_id"`
	}
	if err := eachRow(ctx, q, `SELECT character_id, COALESCE(slots, '') FROM spell_slots`, func(id int64, blob string) error {
		var slots []slot
		if err := decode(blob, &slots); err != nil {
			return fmt.Errorf("spell_slots of character %d: %w", id, err)
		}
		for _, s := range slots {
			add(s.PreparedSpellID, fmt.Sprintf("spell_slots of character %d", id))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Snapshot slots are spellsvc.SpellSlot encoded without JSON tags
	// (prepsvc.SlotsSnapshot).
	type snapSlot struct {
		SpellID string `json:"SpellID"`
	}
	for _, col := range []string{"before_state", "after_state"} {
		query := `SELECT id, COALESCE(` + col + `, '') FROM action_snapshots WHERE state_key = 'spell_slots'`
		if err := eachRow(ctx, q, query, func(id int64, blob string) error {
			var slots []snapSlot
			if err := decode(blob, &slots); err != nil {
				return fmt.Errorf("action_snapshots %d %s: %w", id, col, err)
			}
			for _, s := range slots {
				add(s.SpellID, fmt.Sprintf("action_snapshots %d", id))
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	type payload struct {
		SpellID string `json:"spellId"`
		Spells  []struct {
			ID string `json:"id"`
		} `json:"spells"`
	}
	scan := func(table, query string) error {
		return eachRow(ctx, q, query, func(id int64, blob string) error {
			var p payload
			if err := decode(blob, &p); err != nil {
				return fmt.Errorf("%s %d payload: %w", table, id, err)
			}
			where := fmt.Sprintf("%s %d", table, id)
			add(p.SpellID, where)
			for _, s := range p.Spells {
				add(s.ID, where)
			}
			return nil
		})
	}
	if err := scan("action_log", `SELECT id, COALESCE(payload, '') FROM action_log WHERE kind IN ('cast', 'exposure', 'spell_prep')`); err != nil {
		return nil, err
	}
	if err := scan("action_corrections", `SELECT c.id, COALESCE(c.payload, '') FROM action_corrections c
		JOIN action_log l ON l.id = c.action_id WHERE l.kind IN ('cast', 'exposure', 'spell_prep')`); err != nil {
		return nil, err
	}
	return out, nil
}

// decode reads a JSON column; an empty or null value holds nothing.
func decode(blob string, into any) error {
	if s := strings.TrimSpace(blob); s == "" || s == "null" {
		return nil
	}
	return json.Unmarshal([]byte(blob), into)
}

// eachRow runs fn over (id, text) rows; the rows are read in full first so fn
// may not interleave with an open cursor.
func eachRow(ctx context.Context, q Querier, query string, fn func(int64, string) error) error {
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("scan references: %w", err)
	}
	type row struct {
		id   int64
		blob string
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.blob); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan references: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, r := range all {
		if err := fn(r.id, r.blob); err != nil {
			return err
		}
	}
	return nil
}

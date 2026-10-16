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
	"github.com/ceffo/critical/api/internal/domain/localized"
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
//   - 5.6: the typed stats are parsed from the six raw texts on upsert, and
//     on every start (retypeStats): a parser change re-types the stored
//     stats without a new artifact version or a migration.
func (d *DB) ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	same, err := sameVersion(ctx, d, cat)
	if err != nil {
		return err
	}
	if same {
		return d.retypeStats(ctx)
	}
	return d.WrapTx(ctx, func(q Querier) error {
		if same, err := sameVersion(ctx, q, cat); err != nil || same {
			return err
		}
		recorded, err := recordedCatalogue(ctx, q)
		if err != nil {
			return err
		}
		if err := backfillKeys(ctx, q, cat); err != nil {
			return err
		}
		if err := upsertSpells(ctx, q, cat); err != nil {
			return err
		}
		if err := deleteRemoved(ctx, q, cat, seedRefs, recorded); err != nil {
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
	// The ids were published from the English names: the fallback-language row.
	rows, err := q.QueryContext(ctx, `SELECT s.id, s.key, COALESCE(t.body, ''), s.class, s.level FROM spells s
		LEFT JOIN localized_texts t ON t.text_id = s.name AND t.lang = ? ORDER BY s.id`, localized.Fallback)
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
		if s, ok := artifact[k]; ok && (s.Name.Default() != r.name || s.Class != r.class || s.Level != r.level) {
			mismatches = append(mismatches, fmt.Sprintf("%s: database row %d is %s, the artifact has %s",
				k, r.id, r.triple(), spellRef{name: s.Name.Default(), class: s.Class, level: s.Level}.triple()))
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

// spellTextColumns are the spells columns that key a localized text (ADR
// 0023 step 4), in the order of spellTexts.
const spellTextColumns = `name, description, school, sphere, casting_time, range, duration, area_of_effect,
	components, saving_throw`

// spellTexts are s's texts in the order of spellTextColumns.
func spellTexts(s catalogue.Spell) []localized.Text {
	return []localized.Text{
		s.Name, s.Description, s.School, s.Sphere, s.CastingTime, s.Range, s.Duration, s.AreaOfEffect,
		s.Components, s.SavingThrow,
	}
}

// upsertSpells is step 5.4.2 (with the typed stats of 5.6): insert a new key,
// overwrite every catalogue field of an existing one. Texts go to
// localized_texts (ADR 0023): one row per language present, none for an
// absent one (a language the artifact dropped loses its row, a text it
// dropped loses its key); a spell keeps its text keys across imports and only
// changed rows are written. The typed stats are parsed from the
// fallback-language stat texts.
func upsertSpells(ctx context.Context, q Querier, cat catalogue.Catalogue) error {
	w, err := newTextWriter(ctx, q)
	if err != nil {
		return err
	}
	keys, err := spellTextKeys(ctx, q)
	if err != nil {
		return err
	}
	for _, s := range cat.Spells {
		stats, err := spellStatsJSON(s.CastingTime.Default(), s.Range.Default(), s.Duration.Default(),
			s.AreaOfEffect.Default(), s.Components.Default(), s.SavingThrow.Default())
		if err != nil {
			return fmt.Errorf("spell %q: %w", s.ID, err)
		}
		old := keys[s.ID]
		ids := make([]any, 0, 10)
		for i, t := range spellTexts(s) {
			var prev sql.NullInt64
			if old != nil {
				prev = old[i]
			}
			id, err := w.write(ctx, q, prev, t)
			if err != nil {
				return fmt.Errorf("spell %q: %w", s.ID, err)
			}
			ids = append(ids, id)
		}
		args := append([]any{s.ID, s.Class, s.Level, s.Source, stats}, ids...)
		if _, err := q.ExecContext(ctx, `
			INSERT INTO spells (key, class, level, source, stats, `+spellTextColumns+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET
				class = excluded.class, level = excluded.level, source = excluded.source, stats = excluded.stats,
				name = excluded.name, description = excluded.description, school = excluded.school,
				sphere = excluded.sphere, casting_time = excluded.casting_time, range = excluded.range,
				duration = excluded.duration, area_of_effect = excluded.area_of_effect,
				components = excluded.components, saving_throw = excluded.saving_throw`,
			args...,
		); err != nil {
			return fmt.Errorf("upsert spell %q: %w", s.ID, err)
		}
	}
	return nil
}

// spellTextKeys are the text keys of every keyed spell, in the order of
// spellTextColumns.
func spellTextKeys(ctx context.Context, q Querier) (map[string][]sql.NullInt64, error) {
	rows, err := q.QueryContext(ctx, `SELECT key, `+spellTextColumns+` FROM spells WHERE key IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("read spell text keys: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string][]sql.NullInt64{}
	for rows.Next() {
		var key string
		ids := make([]sql.NullInt64, 10)
		dest := []any{&key}
		for i := range ids {
			dest = append(dest, &ids[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan spell text keys: %w", err)
		}
		out[key] = ids
	}
	return out, rows.Err()
}

// textWriter writes localized texts (ADR 0023) against what is stored: a
// text column is NULL or keys at least one row, so a new key is one more
// than the highest stored.
type textWriter struct {
	stored map[int64]localized.Text
	next   int64
}

func newTextWriter(ctx context.Context, q Querier) (*textWriter, error) {
	rows, err := q.QueryContext(ctx, `SELECT text_id, lang, body FROM localized_texts`)
	if err != nil {
		return nil, fmt.Errorf("read localized texts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	w := &textWriter{stored: map[int64]localized.Text{}, next: 1}
	for rows.Next() {
		var id int64
		var lang, body string
		if err := rows.Scan(&id, &lang, &body); err != nil {
			return nil, fmt.Errorf("scan localized text: %w", err)
		}
		if w.stored[id] == nil {
			w.stored[id] = localized.Text{}
		}
		w.stored[id][lang] = body
		w.next = max(w.next, id+1)
	}
	return w, rows.Err()
}

// write stores t under prev (a new key when prev is NULL) and returns its
// key: NULL, with prev's rows deleted, when t holds no language.
func (w *textWriter) write(ctx context.Context, q Querier, prev sql.NullInt64, t localized.Text) (sql.NullInt64, error) {
	t = t.Clean()
	if len(t) == 0 {
		if prev.Valid {
			if _, err := q.ExecContext(ctx, `DELETE FROM localized_texts WHERE text_id = ?`, prev.Int64); err != nil {
				return sql.NullInt64{}, fmt.Errorf("delete text %d: %w", prev.Int64, err)
			}
			delete(w.stored, prev.Int64)
		}
		return sql.NullInt64{}, nil
	}
	id := prev.Int64
	if !prev.Valid {
		id = w.next
		w.next++
	}
	old := w.stored[id]
	for lang := range old {
		if _, ok := t[lang]; ok {
			continue
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM localized_texts WHERE text_id = ? AND lang = ?`, id, lang); err != nil {
			return sql.NullInt64{}, fmt.Errorf("delete text %d %s: %w", id, lang, err)
		}
	}
	for lang, body := range t {
		if b, ok := old[lang]; ok && b == body {
			continue
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO localized_texts (text_id, lang, body) VALUES (?, ?, ?)
			ON CONFLICT (text_id, lang) DO UPDATE SET body = excluded.body`, id, lang, body); err != nil {
			return sql.NullInt64{}, fmt.Errorf("write text %d %s: %w", id, lang, err)
		}
	}
	w.stored[id] = t
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

// deleteRemoved is step 5.4.3: a keyed spell the artifact no longer holds is
// deleted when nothing refers to it. Any reference fails the import with
// every such key and where it is used.
// recorded names the catalogue the database was built from (recordedCatalogue,
// read before this import keyed anything).
func deleteRemoved(ctx context.Context, q Querier, cat catalogue.Catalogue, seedRefs []catalogue.Ref, recorded string) error {
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
		return fmt.Errorf("spell catalogue: the artifact (version %s) removes spells that are still referenced, refusing to import: %s. "+
			"This database was built from %s: start it with that catalogue (the real one is fetched by `just catalogue` "+
			"to .catalogue/catalogue.json, the default CRITICAL_CATALOGUE), or start this catalogue on a database of its own",
			cat.Version, strings.Join(list, "; "), recorded)
	}
	for k := range removed {
		if _, err := q.ExecContext(ctx, `DELETE FROM localized_texts WHERE text_id IN (
			SELECT j.value FROM spells s, json_each(json_array(`+spellTextColumns+`)) j
			WHERE s.key = ? AND j.value IS NOT NULL)`,
			k); err != nil {
			return fmt.Errorf("delete texts of spell %q: %w", k, err)
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM spells WHERE key = ?`, k); err != nil {
			return fmt.Errorf("delete spell %q: %w", k, err)
		}
	}
	return nil
}

// recordedCatalogue names the catalogue the database was last imported from,
// for the refusal message of deleteRemoved: a database used with the real
// catalogue started on the fixture (or the reverse) is refused there. It is
// read before the import keys anything. Without a recorded version, keyed
// spells mean an earlier import whose record was forgotten (migration 030
// forgot it to re-type the stats); no keyed spell means the spell seeds
// that predate the catalogue artifact.
func recordedCatalogue(ctx context.Context, q Querier) (string, error) {
	var version string
	err := q.QueryRowContext(ctx, `SELECT version FROM catalogue_meta WHERE id = 1`).Scan(&version)
	if err == nil {
		return "catalogue version " + version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read catalogue version: %w", err)
	}
	var keyed int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM spells WHERE key IS NOT NULL`).Scan(&keyed); err != nil {
		return "", fmt.Errorf("count keyed spells: %w", err)
	}
	if keyed > 0 {
		return "an earlier spell catalogue, which holds the spells listed, whose version is not recorded " +
			"(migration 030 forgot it)", nil
	}
	return "the spell seeds that predate the catalogue artifact (the real catalogue's spells)", nil
}

// retypeStats is step 5.6 on a start that imports nothing (5.3): every
// spell's typed stats are parsed again from its stored fallback-language
// stat texts, and a row is written only when they differ from the stored
// ones, so a parser change re-types the database at its next start and an
// unchanged parser writes nothing.
func (d *DB) retypeStats(ctx context.Context) error {
	cols := []string{"casting_time", "range", "duration", "area_of_effect", "components", "saving_throw"}
	sel := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		sel[i] = `COALESCE((SELECT body FROM localized_texts WHERE text_id = s.` + c + ` AND lang = ?), '')`
		args[i] = localized.Fallback
	}
	rows, err := d.QueryContext(ctx, `SELECT s.id, COALESCE(s.stats, ''), `+strings.Join(sel, ", ")+` FROM spells s`, args...)
	if err != nil {
		return fmt.Errorf("read spell stats: %w", err)
	}
	type retyped struct {
		id    int64
		stats string
	}
	var changed []retyped
	for rows.Next() {
		var id int64
		var stored string
		var r [6]string
		if err := rows.Scan(&id, &stored, &r[0], &r[1], &r[2], &r[3], &r[4], &r[5]); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan spell stats: %w", err)
		}
		stats, err := spellStatsJSON(r[0], r[1], r[2], r[3], r[4], r[5])
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("spell %d: %w", id, err)
		}
		if stats != stored {
			changed = append(changed, retyped{id, stats})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(changed) == 0 {
		return nil
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for _, c := range changed {
			if _, err := q.ExecContext(ctx, `UPDATE spells SET stats = ? WHERE id = ?`, c.stats, c.id); err != nil {
				return fmt.Errorf("re-type spell %d: %w", c.id, err)
			}
		}
		return nil
	})
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
// 5.4.3): the prepared spells of spell_slots, the spell_id of the active
// buffs (ADR 0004 step 17), the slot and buff lists in the before/after
// state of action_snapshots (undo and redo restore them), the spellId of
// cast, exposure and buff entries in action_log and of the payloads of their
// corrections, expiredSpellIds of round_advance and time_passed entries, and
// spells[].id of spell_prep entries. A JSON value that
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

	// Buffs are stored and snapshotted with the same spell_id key (buffdal,
	// buffsvc snapBuff).
	type buff struct {
		SpellID string `json:"spell_id"`
	}
	if err := eachRow(ctx, q, `SELECT character_id, COALESCE(buffs, '') FROM active_buffs`, func(id int64, blob string) error {
		var buffs []buff
		if err := decode(blob, &buffs); err != nil {
			return fmt.Errorf("active_buffs of character %d: %w", id, err)
		}
		for _, b := range buffs {
			add(b.SpellID, fmt.Sprintf("active_buffs of character %d", id))
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
		query = `SELECT id, COALESCE(` + col + `, '') FROM action_snapshots WHERE state_key = 'buffs'`
		if err := eachRow(ctx, q, query, func(id int64, blob string) error {
			var buffs []buff
			if err := decode(blob, &buffs); err != nil {
				return fmt.Errorf("action_snapshots %d %s: %w", id, col, err)
			}
			for _, b := range buffs {
				add(b.SpellID, fmt.Sprintf("action_snapshots %d", id))
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
		ExpiredSpellIDs []string `json:"expiredSpellIds"`
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
			for _, id := range p.ExpiredSpellIDs {
				add(id, where)
			}
			return nil
		})
	}
	if err := scan("action_log", `SELECT id, COALESCE(payload, '') FROM action_log WHERE kind IN `+spellRefKinds); err != nil {
		return nil, err
	}
	if err := scan("action_corrections", `SELECT c.id, COALESCE(c.payload, '') FROM action_corrections c
		JOIN action_log l ON l.id = c.action_id WHERE l.kind IN `+spellRefKinds); err != nil {
		return nil, err
	}
	return out, nil
}

// spellRefKinds are the action-log kinds whose payload may hold a spell id.
const spellRefKinds = `('cast', 'exposure', 'spell_prep', 'buff_change', 'round_advance', 'time_passed')`

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

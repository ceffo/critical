package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// ImportCatalogue imports the spell catalogue artifact into the spells table (ADR 0021 step 5,
// after the file was read and validated: steps 5.1 and 5.2 are catalogueadpt's). seedRefs are
// the spell ids the seeds refer to (the spellbook and the spell effects); the ones game state
// holds are read from the database here.
//
//   - 5.3: the artifact version equals the recorded one: nothing is written.
//   - 5.4, in one transaction: 5.4.1 rows without a key are keyed (spellsvc.Keys over the
//     existing rows, the oldest row of a duplicated (name, class, level) only) and each key is
//     verified against the artifact; 5.4.2 every artifact spell is upserted by key; 5.4.3 keyed
//     spells the artifact no longer holds are deleted, unless something refers to them, which
//     rolls the import back.
//   - 5.5: format and version are recorded in the same transaction.
//   - 5.6: the typed stats are parsed from the six raw texts on upsert, and on every start
//     (retypeStats): a parser change re-types the stored stats without a new artifact version or
//     a migration.
func (d *DB) ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	return importCatalogue(ctx, d, d.q, cat, seedRefs)
}

// importCatalogue is ImportCatalogue over its ports: tx runs the transaction, q runs on the
// transaction tx carries.
func importCatalogue(ctx context.Context, tx domain.TxRunner, q storeq.Querier, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	same, err := sameVersion(ctx, q, cat)
	if err != nil {
		return err
	}
	if same {
		return retypeStats(ctx, tx, q)
	}
	return tx.InTx(ctx, func(ctx context.Context) error {
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
		if err := q.RecordCatalogueVersion(ctx, storeq.RecordCatalogueVersionParams{
			Format: int64(cat.Format), Version: cat.Version,
		}); err != nil {
			return fmt.Errorf("record catalogue version: %w", err)
		}
		return nil
	})
}

// sameVersion: step 5.3, equality only (a rollback to an older artifact is an import like any
// other).
func sameVersion(ctx context.Context, q storeq.Querier, cat catalogue.Catalogue) (bool, error) {
	v, err := q.CatalogueVersion(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read catalogue version: %w", err)
	}
	return v.Format == int64(cat.Format) && v.Version == cat.Version, nil
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

// backfillKeys is step 5.4.1: the first import on a database seeded before the catalogue. Keys
// over every existing row reproduces the ids in use; the oldest row of a (name, class, level)
// takes its key, the others stay NULL (never served, never updated). Every key given is checked
// against the artifact spell holding it: a different (name, class, level) means the existing rows
// are not the set the ids were published from, and the import refuses rather than overwrite a
// different spell.
func backfillKeys(ctx context.Context, q storeq.Querier, cat catalogue.Catalogue) error {
	// The ids were published from the English names: the fallback-language row.
	rows, err := q.SpellIdentities(ctx, localized.Fallback)
	if err != nil {
		return fmt.Errorf("read spells: %w", err)
	}
	all := make([]spellRef, len(rows))
	for i, r := range rows {
		all[i] = spellRef{id: r.ID, key: r.Key, name: r.Name, class: r.Class, level: int(r.Level)}
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
	var assign []storeq.SetSpellKeyParams
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
		assign = append(assign, storeq.SetSpellKeyParams{Key: sql.NullString{String: k, Valid: true}, ID: r.id})
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("spell catalogue: the existing spells do not match the artifact's ids, refusing to import: %s",
			strings.Join(mismatches, "; "))
	}
	for _, a := range assign {
		if err := q.SetSpellKey(ctx, a); err != nil {
			return fmt.Errorf("key spell %d: %w", a.ID, err)
		}
	}
	return nil
}

// spellTexts are s's texts in the order of textKeys.
func spellTexts(s catalogue.Spell) []localized.Text {
	return []localized.Text{
		s.Name, s.Description, s.School, s.Sphere, s.CastingTime, s.Range, s.Duration, s.AreaOfEffect,
		s.Components, s.SavingThrow,
	}
}

// textKeys are a spell row's text keys (ADR 0023 step 4), in the order of spellTexts.
func textKeys(r storeq.SpellTextKeysRow) []sql.NullInt64 {
	return []sql.NullInt64{
		r.Name, r.Description, r.School, r.Sphere, r.CastingTime, r.Range, r.Duration, r.AreaOfEffect,
		r.Components, r.SavingThrow,
	}
}

// upsertSpells is step 5.4.2 (with the typed stats of 5.6): insert a new key, overwrite every
// catalogue field of an existing one. Texts go to localized_texts (ADR 0023): one row per
// language present, none for an absent one (a language the artifact dropped loses its row, a
// text it dropped loses its key); a spell keeps its text keys across imports and only changed
// rows are written. The typed stats are parsed from the fallback-language stat texts.
func upsertSpells(ctx context.Context, q storeq.Querier, cat catalogue.Catalogue) error {
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
		ids := make([]sql.NullInt64, 0, 10)
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
		if err := q.UpsertSpell(ctx, storeq.UpsertSpellParams{
			Key: sql.NullString{String: s.ID, Valid: true}, Class: s.Class, Level: int64(s.Level),
			Source: sql.NullString{String: s.Source, Valid: true}, Stats: sql.NullString{String: stats, Valid: true},
			Name: ids[0], Description: ids[1], School: ids[2], Sphere: ids[3], CastingTime: ids[4], Range: ids[5],
			Duration: ids[6], AreaOfEffect: ids[7], Components: ids[8], SavingThrow: ids[9],
		}); err != nil {
			return fmt.Errorf("upsert spell %q: %w", s.ID, err)
		}
	}
	return nil
}

// spellTextKeys are the text keys of every keyed spell, in the order of spellTexts.
func spellTextKeys(ctx context.Context, q storeq.Querier) (map[string][]sql.NullInt64, error) {
	rows, err := q.SpellTextKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("read spell text keys: %w", err)
	}
	out := make(map[string][]sql.NullInt64, len(rows))
	for _, r := range rows {
		out[r.Key] = textKeys(r)
	}
	return out, nil
}

// textWriter writes localized texts (ADR 0023) against what is stored: a text column is NULL or
// keys at least one row, so a new key is one more than the highest stored.
type textWriter struct {
	stored map[int64]localized.Text
	next   int64
}

func newTextWriter(ctx context.Context, q storeq.Querier) (*textWriter, error) {
	rows, err := q.AllTexts(ctx)
	if err != nil {
		return nil, fmt.Errorf("read localized texts: %w", err)
	}
	w := &textWriter{stored: map[int64]localized.Text{}, next: 1}
	for _, r := range rows {
		if w.stored[r.TextID] == nil {
			w.stored[r.TextID] = localized.Text{}
		}
		w.stored[r.TextID][r.Lang] = r.Body
		w.next = max(w.next, r.TextID+1)
	}
	return w, nil
}

// write stores t under prev (a new key when prev is NULL) and returns its key: NULL, with prev's
// rows deleted, when t holds no language.
func (w *textWriter) write(ctx context.Context, q storeq.Querier, prev sql.NullInt64, t localized.Text) (sql.NullInt64, error) {
	t = t.Clean()
	if len(t) == 0 {
		if prev.Valid {
			if err := q.DeleteText(ctx, prev.Int64); err != nil {
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
	for _, lang := range old.Languages() {
		if _, ok := t[lang]; ok {
			continue
		}
		if err := q.DeleteTextLanguage(ctx, storeq.DeleteTextLanguageParams{TextID: id, Lang: lang}); err != nil {
			return sql.NullInt64{}, fmt.Errorf("delete text %d %s: %w", id, lang, err)
		}
	}
	for _, lang := range t.Languages() {
		body := t[lang]
		if b, ok := old[lang]; ok && b == body {
			continue
		}
		if err := q.UpsertText(ctx, storeq.UpsertTextParams{TextID: id, Lang: lang, Body: body}); err != nil {
			return sql.NullInt64{}, fmt.Errorf("write text %d %s: %w", id, lang, err)
		}
	}
	w.stored[id] = t
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

// deleteRemoved is step 5.4.3: a keyed spell the artifact no longer holds is deleted, with its
// texts, when nothing refers to it. Any reference fails the import with every such key and where
// it is used. recorded names the catalogue the database was built from (recordedCatalogue, read
// before this import keyed anything).
func deleteRemoved(ctx context.Context, q storeq.Querier, cat catalogue.Catalogue, seedRefs []catalogue.Ref, recorded string) error {
	keys, err := spellTextKeys(ctx, q)
	if err != nil {
		return err
	}
	artifact := cat.ByID()
	removed := map[string]bool{}
	for k := range keys {
		if _, ok := artifact[k]; !ok {
			removed[k] = true
		}
	}
	if len(removed) == 0 {
		return nil
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
		used := make([]string, 0, len(uses))
		for k := range uses {
			used = append(used, k)
		}
		sort.Strings(used)
		list := make([]string, len(used))
		for i, k := range used {
			list[i] = k + " (" + strings.Join(uses[k], ", ") + ")"
		}
		return fmt.Errorf("spell catalogue: the artifact (version %s) removes spells that are still referenced, refusing to import: %s. "+
			"This database was built from %s: start it with that catalogue (the real one is fetched by `just catalogue` "+
			"to .catalogue/catalogue.json, the default CRITICAL_CATALOGUE), or start this catalogue on a database of its own",
			cat.Version, strings.Join(list, "; "), recorded)
	}
	gone := make([]string, 0, len(removed))
	for k := range removed {
		gone = append(gone, k)
	}
	sort.Strings(gone)
	for _, k := range gone {
		// The row first, then the texts it keyed: nothing is left pointing at a deleted text.
		if err := q.DeleteSpell(ctx, sql.NullString{String: k, Valid: true}); err != nil {
			return fmt.Errorf("delete spell %q: %w", k, err)
		}
		for _, id := range keys[k] {
			if !id.Valid {
				continue
			}
			if err := q.DeleteText(ctx, id.Int64); err != nil {
				return fmt.Errorf("delete texts of spell %q: %w", k, err)
			}
		}
	}
	return nil
}

// recordedCatalogue names the catalogue the database was last imported from, for the refusal
// message of deleteRemoved: a database used with the real catalogue started on the fixture (or
// the reverse) is refused there. It is read before the import keys anything. Without a recorded
// version, keyed spells mean an earlier import whose record was forgotten (an upgrade of the
// SQLite schema cleared it to re-type the stats); no keyed spell means the spell seeds that
// predate the catalogue artifact.
func recordedCatalogue(ctx context.Context, q storeq.Querier) (string, error) {
	v, err := q.CatalogueVersion(ctx)
	if err == nil {
		return "catalogue version " + v.Version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read catalogue version: %w", err)
	}
	keyed, err := q.CountKeyedSpells(ctx)
	if err != nil {
		return "", fmt.Errorf("count keyed spells: %w", err)
	}
	if keyed > 0 {
		return "an earlier spell catalogue, which holds the spells listed, whose version is not recorded " +
			"(an upgrade of the database cleared it)", nil
	}
	return "the spell seeds that predate the catalogue artifact (the real catalogue's spells)", nil
}

// retypeStats is step 5.6 on a start that imports nothing (5.3): every spell's typed stats are
// parsed again from its stored fallback-language stat texts, and a row is written only when they
// differ from the stored ones, so a parser change re-types the database at its next start and an
// unchanged parser writes nothing.
func retypeStats(ctx context.Context, tx domain.TxRunner, q storeq.Querier) error {
	rows, err := q.SpellStatTexts(ctx, localized.Fallback)
	if err != nil {
		return fmt.Errorf("read spell stats: %w", err)
	}
	var changed []storeq.SetSpellStatsParams
	for _, r := range rows {
		stats, err := spellStatsJSON(r.CastingTime, r.Range, r.Duration, r.AreaOfEffect, r.Components, r.SavingThrow)
		if err != nil {
			return fmt.Errorf("spell %d: %w", r.ID, err)
		}
		if stats != r.Stats {
			changed = append(changed, storeq.SetSpellStatsParams{Stats: sql.NullString{String: stats, Valid: true}, ID: r.ID})
		}
	}
	if len(changed) == 0 {
		return nil
	}
	return tx.InTx(ctx, func(ctx context.Context) error {
		for _, c := range changed {
			if err := q.SetSpellStats(ctx, c); err != nil {
				return fmt.Errorf("re-type spell %d: %w", c.ID, err)
			}
		}
		return nil
	})
}

// spellRefKinds are the action-log kinds whose payload may hold a spell id.
var spellRefKinds = []string{"cast", "exposure", "spell_prep", "buff_change", "round_advance", "time_passed", "spell_learn"}

// spellbookSnapshotPrefix starts the state key of a spellbook snapshot, "spellbook:<spell key>"
// (learnsvc, ADR 0030 step 20.3).
const spellbookSnapshotPrefix = "spellbook:"

// StateSpellRefs returns every spell id game state holds (ADR 0021 step 5.4.3): the spellbook
// entries and the failed learns (spellbook_learn_failures) of every character, the spellId of the
// "spellbook:<key>" snapshots and of spell_learn entries (ADR 0030 step 20), the prepared spells of spell_slots, the spell_id of the active buffs (ADR 0004 step 17), the slot and buff
// lists in the before/after state of action_snapshots (undo and redo restore them), the spellId
// of cast, exposure and buff entries in action_log and of the payloads of their corrections,
// expiredSpellIds of round_advance and time_passed entries, and spells[].id of spell_prep
// entries. A JSON value that does not decode is an error: an unreadable reference is not a
// missing one.
func StateSpellRefs(ctx context.Context, q storeq.Querier) ([]catalogue.Ref, error) {
	var out []catalogue.Ref
	add := func(id, where string) {
		if id != "" {
			out = append(out, catalogue.Ref{ID: id, Where: where})
		}
	}

	type slot struct {
		PreparedSpellID string `json:"prepared_spell_id"`
	}
	book, err := q.SpellbookRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range book {
		add(r.SpellKey, fmt.Sprintf("spellbook of character %d", r.CharacterID))
	}

	failures, err := q.LearnFailureRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range failures {
		add(r.SpellKey, fmt.Sprintf("spellbook_learn_failures of character %d", r.CharacterID))
	}

	// A spellbook snapshot names its spell in the state (learnsvc spellState) and in its key.
	type bookState struct {
		SpellID string `json:"spellId"`
	}
	bookSnaps, err := q.SnapshotsWithPrefix(ctx, spellbookSnapshotPrefix)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range bookSnaps {
		where := fmt.Sprintf("action_snapshots %d", r.ID)
		for _, c := range [][2]string{{"before_state", r.BeforeState}, {"after_state", r.AfterState}} {
			col, blob := c[0], c[1]
			var st bookState
			if err := decode(blob, &st); err != nil {
				return nil, fmt.Errorf("action_snapshots %d %s: %w", r.ID, col, err)
			}
			add(st.SpellID, where)
		}
	}

	slotLists, err := q.SpellSlotLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range slotLists {
		var slots []slot
		if err := decode(r.Slots, &slots); err != nil {
			return nil, fmt.Errorf("spell_slots of character %d: %w", r.CharacterID, err)
		}
		for _, s := range slots {
			add(s.PreparedSpellID, fmt.Sprintf("spell_slots of character %d", r.CharacterID))
		}
	}

	// Buffs are stored and snapshotted with the same spell_id key (buffdal, buffsvc snapBuff).
	type buff struct {
		SpellID string `json:"spell_id"`
	}
	buffLists, err := q.ActiveBuffLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range buffLists {
		var buffs []buff
		if err := decode(r.Buffs, &buffs); err != nil {
			return nil, fmt.Errorf("active_buffs of character %d: %w", r.CharacterID, err)
		}
		for _, b := range buffs {
			add(b.SpellID, fmt.Sprintf("active_buffs of character %d", r.CharacterID))
		}
	}

	// Snapshot slots are spellsvc.SpellSlot encoded without JSON tags (prepsvc.SlotsSnapshot).
	type snapSlot struct {
		SpellID string `json:"SpellID"`
	}
	for _, kind := range []string{"spell_slots", "buffs"} {
		snaps, err := q.SnapshotsOf(ctx, kind)
		if err != nil {
			return nil, fmt.Errorf("scan references: %w", err)
		}
		for _, col := range []string{"before_state", "after_state"} {
			for _, r := range snaps {
				blob := r.BeforeState
				if col == "after_state" {
					blob = r.AfterState
				}
				where := fmt.Sprintf("action_snapshots %d", r.ID)
				if kind == "spell_slots" {
					var slots []snapSlot
					if err := decode(blob, &slots); err != nil {
						return nil, fmt.Errorf("action_snapshots %d %s: %w", r.ID, col, err)
					}
					for _, s := range slots {
						add(s.SpellID, where)
					}
					continue
				}
				var buffs []buff
				if err := decode(blob, &buffs); err != nil {
					return nil, fmt.Errorf("action_snapshots %d %s: %w", r.ID, col, err)
				}
				for _, b := range buffs {
					add(b.SpellID, where)
				}
			}
		}
	}

	type payload struct {
		SpellID string `json:"spellId"`
		Spells  []struct {
			ID string `json:"id"`
		} `json:"spells"`
		ExpiredSpellIDs []string `json:"expiredSpellIds"`
	}
	scan := func(table string, id int64, blob string) error {
		var p payload
		if err := decode(blob, &p); err != nil {
			return fmt.Errorf("%s %d payload: %w", table, id, err)
		}
		where := fmt.Sprintf("%s %d", table, id)
		add(p.SpellID, where)
		for _, s := range p.Spells {
			add(s.ID, where)
		}
		for _, e := range p.ExpiredSpellIDs {
			add(e, where)
		}
		return nil
	}
	entries, err := q.SpellEntryPayloads(ctx, spellRefKinds)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range entries {
		if err := scan("action_log", r.ID, r.Payload); err != nil {
			return nil, err
		}
	}
	corrections, err := q.SpellCorrectionPayloads(ctx, spellRefKinds)
	if err != nil {
		return nil, fmt.Errorf("scan references: %w", err)
	}
	for _, r := range corrections {
		if err := scan("action_corrections", r.ID, r.Payload); err != nil {
			return nil, err
		}
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

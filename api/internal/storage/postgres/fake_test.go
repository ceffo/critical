package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/ceffo/critical/api/internal/domain/localized"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// fakeStore stands in for the database behind the storage's sqlc Querier (ADR 0025 step 6.2: no
// test opens a database). It keeps the tables the seeds and the catalogue import read and
// write, enforces what the schema would refuse (a duplicate key, a row whose character does not
// exist, an empty text body, the CHECKs of weapons and items) and runs InTx all or nothing.
// Weapons, items, proficiencies and bard skills are keyed by id, as their primary keys still are
// (migration 002); their CharacterID 0 is a NULL owner (a row from before owners, or written by
// the release before during a rolling deploy). The seed and import tests drive the
// real code over it and read its tables, so they assert what is stored, not which SQL ran.
type fakeStore struct {
	t tables
}

// tables is the fake's state; JSON copies it for InTx's rollback.
type tables struct {
	Characters []int64
	Names      map[int64]string
	Slots      map[int64]string
	Buffs      map[int64]string
	Rounds     map[int64]bool
	Weapons    map[string]storeq.InsertWeaponParams
	Items      map[string]storeq.InsertItemParams
	// Offers are seeded_name_languages rows, "kind|id|lang".
	Offers map[string]bool
	Profs  map[string]storeq.UpsertProficiencyParams
	Bard   map[string]storeq.UpsertBardSkillParams
	// Spellbook is spellbook_entries, by spellbookKey.
	Spellbook map[string]storeq.UpsertSpellbookEntryParams
	Texts     map[int64]localized.Text
	Meta      *storeq.CatalogueVersionRow
	Spells    []spellRow
	// Game state that refers to spells.
	Snapshots   []snapshotRow
	Log         []logRow
	Corrections []correctionRow
}

type spellRow struct {
	ID     int64
	Key    sql.NullString
	Class  string
	Level  int64
	Source sql.NullString
	Stats  sql.NullString
	Keys   [10]sql.NullInt64
}

type snapshotRow struct {
	ID            int64
	ActionID      int64
	Key           string
	Before, After string
}

type logRow struct {
	ID      int64
	Kind    string
	Payload string
}

type correctionRow struct {
	ID       int64
	ActionID int64
	Payload  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{t: tables{
		Slots: map[int64]string{}, Buffs: map[int64]string{}, Rounds: map[int64]bool{},
		Weapons: map[string]storeq.InsertWeaponParams{}, Items: map[string]storeq.InsertItemParams{},
		Offers: map[string]bool{}, Profs: map[string]storeq.UpsertProficiencyParams{},
		Bard: map[string]storeq.UpsertBardSkillParams{}, Texts: map[int64]localized.Text{},
		Names: map[int64]string{}, Spellbook: map[string]storeq.UpsertSpellbookEntryParams{},
	}}
}

var _ storeq.Querier = (*fakeStore)(nil)

// copyTables is a deep copy of t.
func copyTables(t tables) tables {
	raw, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	var out tables
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// InTx runs fn all or nothing: a failure restores the tables as they were.
func (f *fakeStore) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	saved := copyTables(f.t)
	defer func() {
		if p := recover(); p != nil {
			f.t = saved
			panic(p)
		}
		if err != nil {
			f.t = saved
		}
	}()
	return fn(ctx)
}

func (f *fakeStore) hasCharacter(id int64) error {
	if !slices.Contains(f.t.Characters, id) {
		return fmt.Errorf("foreign key violation: no character %d", id)
	}
	return nil
}

func (f *fakeStore) spell(key sql.NullString) *spellRow {
	for i := range f.t.Spells {
		if f.t.Spells[i].Key.Valid && key.Valid && f.t.Spells[i].Key.String == key.String {
			return &f.t.Spells[i]
		}
	}
	return nil
}

func (f *fakeStore) body(key sql.NullInt64, lang string) string {
	if !key.Valid {
		return ""
	}
	return f.t.Texts[key.Int64][lang]
}

func (f *fakeStore) LockWriters(context.Context) error { return nil }

func (f *fakeStore) CountCharacters(context.Context) (int64, error) {
	return int64(len(f.t.Characters)), nil
}

func (f *fakeStore) InsertCharacter(_ context.Context, arg storeq.InsertCharacterParams) (int64, error) {
	id := int64(1)
	for _, c := range f.t.Characters {
		id = max(id, c+1)
	}
	f.t.Characters = append(f.t.Characters, id)
	f.t.Names[id] = arg.Name
	return id, nil
}

func (f *fakeStore) SeedCharacterID(_ context.Context, name string) (int64, error) {
	for _, id := range f.t.Characters { // ascending: ids are appended in order
		if f.t.Names[id] == name {
			return id, nil
		}
	}
	return 0, sql.ErrNoRows
}

func (f *fakeStore) CountUnowned(context.Context) (int64, error) {
	var n int64
	for _, w := range f.t.Weapons {
		n += unowned(w.CharacterID)
	}
	for _, it := range f.t.Items {
		n += unowned(it.CharacterID)
	}
	for _, p := range f.t.Profs {
		n += unowned(p.CharacterID)
	}
	for _, b := range f.t.Bard {
		n += unowned(b.CharacterID)
	}
	return n, nil
}

func unowned(characterID int64) int64 {
	if characterID == 0 {
		return 1
	}
	return 0
}

func (f *fakeStore) AdoptWeapons(_ context.Context, characterID int64) error {
	return adopt(f, f.t.Weapons, characterID, func(w *storeq.InsertWeaponParams) *int64 { return &w.CharacterID })
}

func (f *fakeStore) AdoptItems(_ context.Context, characterID int64) error {
	return adopt(f, f.t.Items, characterID, func(it *storeq.InsertItemParams) *int64 { return &it.CharacterID })
}

func (f *fakeStore) AdoptProficiencies(_ context.Context, characterID int64) error {
	return adopt(f, f.t.Profs, characterID, func(p *storeq.UpsertProficiencyParams) *int64 { return &p.CharacterID })
}

func (f *fakeStore) AdoptBardSkills(_ context.Context, characterID int64) error {
	return adopt(f, f.t.Bard, characterID, func(b *storeq.UpsertBardSkillParams) *int64 { return &b.CharacterID })
}

// adopt gives every row of table without an owner to characterID (the foreign key holds).
func adopt[R any](f *fakeStore, table map[string]R, characterID int64, owner func(*R) *int64) error {
	if err := f.hasCharacter(characterID); err != nil {
		return err
	}
	for id, r := range table {
		if o := owner(&r); *o == 0 {
			*o = characterID
			table[id] = r
		}
	}
	return nil
}

func (f *fakeStore) InsertSpellSlots(_ context.Context, arg storeq.InsertSpellSlotsParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	f.t.Slots[arg.CharacterID] = arg.Slots
	return nil
}

func (f *fakeStore) InsertActiveBuffs(_ context.Context, characterID int64) error {
	if err := f.hasCharacter(characterID); err != nil {
		return err
	}
	f.t.Buffs[characterID] = "[]"
	return nil
}

func (f *fakeStore) InsertRoundState(_ context.Context, characterID int64) error {
	if err := f.hasCharacter(characterID); err != nil {
		return err
	}
	f.t.Rounds[characterID] = true
	return nil
}

func (f *fakeStore) CountWeapons(_ context.Context, characterID int64) (int64, error) {
	return count(f.t.Weapons, func(w storeq.InsertWeaponParams) bool { return w.CharacterID == characterID }), nil
}

func (f *fakeStore) CountItems(_ context.Context, characterID int64) (int64, error) {
	return count(f.t.Items, func(it storeq.InsertItemParams) bool { return it.CharacterID == characterID }), nil
}

func (f *fakeStore) CountSkills(_ context.Context, characterID int64) (int64, error) {
	return count(f.t.Profs, func(p storeq.UpsertProficiencyParams) bool { return p.CharacterID == characterID }) +
		count(f.t.Bard, func(b storeq.UpsertBardSkillParams) bool { return b.CharacterID == characterID }), nil
}

func count[R any](table map[string]R, keep func(R) bool) int64 {
	var n int64
	for _, r := range table {
		if keep(r) {
			n++
		}
	}
	return n
}

func (f *fakeStore) InsertWeapon(_ context.Context, arg storeq.InsertWeaponParams) error {
	if !slices.Contains([]string{"melee", "missile"}, arg.Kind) || (arg.TwoHanded != 0 && arg.TwoHanded != 1) {
		return fmt.Errorf("check violation: weapon %q kind %q two_handed %d", arg.ID, arg.Kind, arg.TwoHanded)
	}
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	if _, ok := f.t.Weapons[arg.ID]; ok {
		return fmt.Errorf("duplicate key weapons %q", arg.ID)
	}
	f.t.Weapons[arg.ID] = arg
	return nil
}

func (f *fakeStore) InsertItem(_ context.Context, arg storeq.InsertItemParams) error {
	if !slices.Contains([]string{"stock", "charges", "daily"}, arg.Behaviour) || arg.Count < 0 || arg.MaxCount < 0 {
		return fmt.Errorf("check violation: item %q behaviour %q count %d max %d", arg.ID, arg.Behaviour, arg.Count, arg.MaxCount)
	}
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	if _, ok := f.t.Items[arg.ID]; ok {
		return fmt.Errorf("duplicate key items %q", arg.ID)
	}
	f.t.Items[arg.ID] = arg
	return nil
}

func (f *fakeStore) WeaponNameKey(_ context.Context, arg storeq.WeaponNameKeyParams) (int64, error) {
	w, ok := f.t.Weapons[arg.ID]
	if !ok || w.CharacterID != arg.CharacterID {
		return 0, sql.ErrNoRows
	}
	return w.Name, nil
}

func (f *fakeStore) ItemNameKey(_ context.Context, arg storeq.ItemNameKeyParams) (int64, error) {
	it, ok := f.t.Items[arg.ID]
	if !ok || it.CharacterID != arg.CharacterID {
		return 0, sql.ErrNoRows
	}
	return it.Name, nil
}

func offer(kind, id, lang string) string { return kind + "|" + id + "|" + lang }

func (f *fakeStore) SeedOffers(_ context.Context, arg storeq.SeedOffersParams) ([]string, error) {
	var out []string
	for _, lang := range []string{"en", "fr", "de", "es"} {
		if f.t.Offers[offer(arg.Kind, arg.ID, lang)] {
			out = append(out, lang)
		}
	}
	return out, nil
}

func (f *fakeStore) RecordSeedOffer(_ context.Context, arg storeq.RecordSeedOfferParams) error {
	f.t.Offers[offer(arg.Kind, arg.ID, arg.Lang)] = true
	return nil
}

// The skill upserts conflict on (character_id, id); the primary key on id alone still refuses
// another character's row of the same id.
func (f *fakeStore) UpsertProficiency(_ context.Context, arg storeq.UpsertProficiencyParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	if cur, ok := f.t.Profs[arg.ID]; ok && cur.CharacterID != arg.CharacterID {
		return fmt.Errorf("duplicate key proficiencies %q", arg.ID)
	}
	f.t.Profs[arg.ID] = arg
	return nil
}

func (f *fakeStore) UpsertBardSkill(_ context.Context, arg storeq.UpsertBardSkillParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	if cur, ok := f.t.Bard[arg.ID]; ok && cur.CharacterID != arg.CharacterID {
		return fmt.Errorf("duplicate key bard_skills %q", arg.ID)
	}
	f.t.Bard[arg.ID] = arg
	return nil
}

func spellbookKey(characterID int64, spellKey string) string {
	return fmt.Sprintf("%d|%s", characterID, spellKey)
}

func (f *fakeStore) UpsertSpellbookEntry(_ context.Context, arg storeq.UpsertSpellbookEntryParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	f.t.Spellbook[spellbookKey(arg.CharacterID, arg.SpellKey)] = arg
	return nil
}

func (f *fakeStore) SpellbookRefs(context.Context) ([]storeq.SpellbookRefsRow, error) {
	rows := make([]storeq.UpsertSpellbookEntryParams, 0, len(f.t.Spellbook))
	for _, e := range f.t.Spellbook {
		rows = append(rows, e)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.CharacterID != b.CharacterID {
			return a.CharacterID < b.CharacterID
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.SpellKey < b.SpellKey
	})
	out := []storeq.SpellbookRefsRow{}
	for _, e := range rows {
		out = append(out, storeq.SpellbookRefsRow{CharacterID: e.CharacterID, SpellKey: e.SpellKey})
	}
	return out, nil
}

// spellbookOf is the character's spellbook keys in its order.
func (f *fakeStore) spellbookOf(characterID int64) []string {
	rows, _ := f.SpellbookRefs(context.Background())
	out := []string{}
	for _, r := range rows {
		if r.CharacterID == characterID {
			out = append(out, r.SpellKey)
		}
	}
	return out
}

func (f *fakeStore) NextTextKey(context.Context) (int64, error) {
	next := int64(1)
	for id := range f.t.Texts {
		next = max(next, id+1)
	}
	return next, nil
}

func (f *fakeStore) TextLanguages(_ context.Context, textID int64) ([]storeq.TextLanguagesRow, error) {
	out := []storeq.TextLanguagesRow{}
	for _, lang := range f.t.Texts[textID].Languages() {
		out = append(out, storeq.TextLanguagesRow{Lang: lang, Body: f.t.Texts[textID][lang]})
	}
	return out, nil
}

func (f *fakeStore) AllTexts(context.Context) ([]storeq.LocalizedText, error) {
	out := []storeq.LocalizedText{}
	for id, t := range f.t.Texts {
		for lang, body := range t {
			out = append(out, storeq.LocalizedText{TextID: id, Lang: lang, Body: body})
		}
	}
	return out, nil
}

func (f *fakeStore) putText(id int64, lang, body string, replace bool) error {
	if body == "" {
		return fmt.Errorf("check violation: empty body for text %d %s", id, lang)
	}
	if _, ok := f.t.Texts[id][lang]; ok && !replace {
		return fmt.Errorf("duplicate key localized_texts (%d, %s)", id, lang)
	}
	if f.t.Texts[id] == nil {
		f.t.Texts[id] = localized.Text{}
	}
	f.t.Texts[id][lang] = body
	return nil
}

func (f *fakeStore) InsertText(_ context.Context, arg storeq.InsertTextParams) error {
	return f.putText(arg.TextID, arg.Lang, arg.Body, false)
}

func (f *fakeStore) UpsertText(_ context.Context, arg storeq.UpsertTextParams) error {
	return f.putText(arg.TextID, arg.Lang, arg.Body, true)
}

func (f *fakeStore) DeleteText(_ context.Context, textID int64) error {
	delete(f.t.Texts, textID)
	return nil
}

func (f *fakeStore) DeleteTextLanguage(_ context.Context, arg storeq.DeleteTextLanguageParams) error {
	delete(f.t.Texts[arg.TextID], arg.Lang)
	if len(f.t.Texts[arg.TextID]) == 0 {
		delete(f.t.Texts, arg.TextID)
	}
	return nil
}

func (f *fakeStore) CatalogueVersion(context.Context) (storeq.CatalogueVersionRow, error) {
	if f.t.Meta == nil {
		return storeq.CatalogueVersionRow{}, sql.ErrNoRows
	}
	return *f.t.Meta, nil
}

func (f *fakeStore) RecordCatalogueVersion(_ context.Context, arg storeq.RecordCatalogueVersionParams) error {
	f.t.Meta = &storeq.CatalogueVersionRow{Format: arg.Format, Version: arg.Version}
	return nil
}

func (f *fakeStore) CountKeyedSpells(context.Context) (int64, error) {
	var n int64
	for _, s := range f.t.Spells {
		if s.Key.Valid {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) SpellIdentities(_ context.Context, lang string) ([]storeq.SpellIdentitiesRow, error) {
	out := []storeq.SpellIdentitiesRow{}
	for _, s := range f.sortedSpells() {
		out = append(out, storeq.SpellIdentitiesRow{
			ID: s.ID, Key: s.Key, Name: f.body(s.Keys[0], lang), Class: s.Class, Level: s.Level,
		})
	}
	return out, nil
}

func (f *fakeStore) sortedSpells() []spellRow {
	out := append([]spellRow(nil), f.t.Spells...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (f *fakeStore) SetSpellKey(_ context.Context, arg storeq.SetSpellKeyParams) error {
	if f.spell(arg.Key) != nil {
		return fmt.Errorf("duplicate key spells %q", arg.Key.String)
	}
	for i := range f.t.Spells {
		if f.t.Spells[i].ID == arg.ID {
			f.t.Spells[i].Key = arg.Key
		}
	}
	return nil
}

func (f *fakeStore) SpellTextKeys(context.Context) ([]storeq.SpellTextKeysRow, error) {
	out := []storeq.SpellTextKeysRow{}
	for _, s := range f.t.Spells {
		if !s.Key.Valid {
			continue
		}
		k := s.Keys
		out = append(out, storeq.SpellTextKeysRow{
			Key: s.Key.String, Name: k[0], Description: k[1], School: k[2], Sphere: k[3], CastingTime: k[4],
			Range: k[5], Duration: k[6], AreaOfEffect: k[7], Components: k[8], SavingThrow: k[9],
		})
	}
	return out, nil
}

func (f *fakeStore) UpsertSpell(_ context.Context, arg storeq.UpsertSpellParams) error {
	keys := [10]sql.NullInt64{
		arg.Name, arg.Description, arg.School, arg.Sphere, arg.CastingTime, arg.Range, arg.Duration,
		arg.AreaOfEffect, arg.Components, arg.SavingThrow,
	}
	if s := f.spell(arg.Key); s != nil {
		s.Class, s.Level, s.Source, s.Stats, s.Keys = arg.Class, arg.Level, arg.Source, arg.Stats, keys
		return nil
	}
	id := int64(1)
	for _, s := range f.t.Spells {
		id = max(id, s.ID+1)
	}
	f.t.Spells = append(f.t.Spells, spellRow{
		ID: id, Key: arg.Key, Class: arg.Class, Level: arg.Level, Source: arg.Source, Stats: arg.Stats, Keys: keys,
	})
	return nil
}

func (f *fakeStore) DeleteSpell(_ context.Context, key sql.NullString) error {
	f.t.Spells = slices.DeleteFunc(f.t.Spells, func(s spellRow) bool {
		return s.Key.Valid && key.Valid && s.Key.String == key.String
	})
	return nil
}

func (f *fakeStore) SpellStatTexts(_ context.Context, lang string) ([]storeq.SpellStatTextsRow, error) {
	out := []storeq.SpellStatTextsRow{}
	for _, s := range f.sortedSpells() {
		out = append(out, storeq.SpellStatTextsRow{
			ID: s.ID, Stats: s.Stats.String, CastingTime: f.body(s.Keys[4], lang), Range: f.body(s.Keys[5], lang),
			Duration: f.body(s.Keys[6], lang), AreaOfEffect: f.body(s.Keys[7], lang),
			Components: f.body(s.Keys[8], lang), SavingThrow: f.body(s.Keys[9], lang),
		})
	}
	return out, nil
}

func (f *fakeStore) SetSpellStats(_ context.Context, arg storeq.SetSpellStatsParams) error {
	for i := range f.t.Spells {
		if f.t.Spells[i].ID == arg.ID {
			f.t.Spells[i].Stats = arg.Stats
		}
	}
	return nil
}

func (f *fakeStore) SpellSlotLists(context.Context) ([]storeq.SpellSlotListsRow, error) {
	out := []storeq.SpellSlotListsRow{}
	for _, id := range f.t.Characters {
		if s, ok := f.t.Slots[id]; ok {
			out = append(out, storeq.SpellSlotListsRow{CharacterID: id, Slots: s})
		}
	}
	return out, nil
}

func (f *fakeStore) ActiveBuffLists(context.Context) ([]storeq.ActiveBuffListsRow, error) {
	out := []storeq.ActiveBuffListsRow{}
	for _, id := range f.t.Characters {
		if b, ok := f.t.Buffs[id]; ok {
			out = append(out, storeq.ActiveBuffListsRow{CharacterID: id, Buffs: b})
		}
	}
	return out, nil
}

func (f *fakeStore) SnapshotsOf(_ context.Context, stateKey string) ([]storeq.SnapshotsOfRow, error) {
	out := []storeq.SnapshotsOfRow{}
	for _, s := range f.t.Snapshots {
		if s.Key == stateKey {
			out = append(out, storeq.SnapshotsOfRow{ID: s.ID, BeforeState: s.Before, AfterState: s.After})
		}
	}
	return out, nil
}

func (f *fakeStore) SpellEntryPayloads(_ context.Context, kinds []string) ([]storeq.SpellEntryPayloadsRow, error) {
	out := []storeq.SpellEntryPayloadsRow{}
	for _, e := range f.t.Log {
		if slices.Contains(kinds, e.Kind) {
			out = append(out, storeq.SpellEntryPayloadsRow{ID: e.ID, Payload: e.Payload})
		}
	}
	return out, nil
}

func (f *fakeStore) SpellCorrectionPayloads(_ context.Context, kinds []string) ([]storeq.SpellCorrectionPayloadsRow, error) {
	out := []storeq.SpellCorrectionPayloadsRow{}
	for _, c := range f.t.Corrections {
		for _, e := range f.t.Log {
			if e.ID == c.ActionID && slices.Contains(kinds, e.Kind) {
				out = append(out, storeq.SpellCorrectionPayloadsRow{ID: c.ID, Payload: c.Payload})
			}
		}
	}
	return out, nil
}

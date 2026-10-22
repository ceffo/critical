package postgres

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// fakeStore stands in for the database behind the storage's sqlc Querier (ADR 0025 step 6.2: no
// test opens a database). It keeps the tables the seeds and the catalogue import read and
// write, enforces what the schema would refuse (a duplicate key, a row whose character does not
// exist, an empty text body, the CHECKs of weapons and items) and runs InTx all or nothing.
// Weapons, items, proficiencies and bard skills are keyed by (character, id), rowKey, as their
// primary keys are (migration 004): two characters may hold the same id. Their CharacterID 0 is a
// NULL owner, which only the database before migration 004 holds (ownRows' tests: a row from
// before owners, or written by a release without them). The seed and import tests drive the
// real code over it and read its tables, so they assert what is stored, not which SQL ran.
type fakeStore struct {
	t tables
}

// tables is the fake's state; JSON copies it for InTx's rollback.
type tables struct {
	Characters []int64
	Names      map[int64]string
	// Races are characters.race_code ('' none, the column's default).
	Races map[int64]string
	// SkillPoints are characters.skill_points ('{}' none, the column's default).
	SkillPoints map[int64]string
	// SkillSets are characters.skill_sets ('{}' none, the column's default).
	SkillSets map[int64]string
	// Picks are characters.proficiency_picks (absent: '{}', the column's default).
	Picks map[int64]string
	// Classes are characters.class (absent: 'bard', the column's default); SkillArmors
	// characters.skill_armor (absent: '', the column's default).
	Classes, SkillArmors map[int64]string
	// Alignments and Sexes are characters.alignment and characters.sex ('' missing).
	Alignments, Sexes map[int64]string
	// Languages are characters.languages, a JSON list.
	Languages map[int64]string
	Slots     map[int64]string
	Buffs     map[int64]string
	Rounds    map[int64]bool
	// Weapons, Items, Profs and Bard are by rowKey. Weapons is the weapons table as stored before
	// the equipment model (migration 016), which only the migration run reads.
	Weapons map[string]weaponRow
	Items   map[string]storeq.InsertItemParams
	// Reviews are equipment_review rows, "character|item|reason"; Renames item_id_renames, by rowKey
	// of the old id.
	Reviews map[string]bool
	Renames map[string]string
	// SnapshotKeys are the state keys of the action log's snapshots, by character; AttackAmmo the
	// attack form's chosen ammunition item id, by character.
	SnapshotKeys map[int64][]string
	AttackAmmo   map[int64]string
	// Offers are seeded_name_languages rows, "kind|id|lang".
	Offers map[string]bool
	Profs  map[string]storeq.UpsertProficiencyParams
	Bard   map[string]storeq.UpsertBardSkillParams
	// Spellbook is spellbook_entries, by spellbookKey.
	Spellbook map[string]storeq.InsertSpellbookEntryParams
	// LearnFailures is spellbook_learn_failures (ADR 0030 step 20).
	LearnFailures []storeq.LearnFailureRefsRow
	Texts         map[int64]localized.Text
	// ProfDescs are proficiency_descriptions rows: the proficiency id to its text key.
	ProfDescs map[string]int64
	Meta      *storeq.CatalogueVersionRow
	Spells    []spellRow
	// Game state that refers to spells.
	Snapshots   []snapshotRow
	Log         []logRow
	Corrections []correctionRow
	// Rulesets are the ruleset tables and the legacy crit_table_rows (ADR 0041 step 10).
	Rulesets rulesetTables
}

// weaponRow is a weapons row (the table no query writes since migration 016).
type weaponRow struct {
	CharacterID int64
	ID          string
	Position    int64
	Name        int64
	DamageSm    string
	DamageL     string
	MagicToHit  int64
	MagicDamage int64
	Kind        string
	TwoHanded   int64
	AmmoKind    string
	WeaponType  string
	Specialized int64
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

// snapshotRow is an action_snapshots row: Before and After are the columns written before the
// codec's envelopes (migration 019), BeforeEnv and AfterEnv the envelopes ("": NULL, not
// re-encoded yet); CharacterID is its entry's character (action_log.character_id).
type snapshotRow struct {
	ID                  int64
	ActionID            int64
	Key                 string
	Before, After       string
	BeforeEnv, AfterEnv string
	CharacterID         int64
}

type logRow struct {
	ID          int64
	CharacterID int64
	Kind        string
	Payload     string
}

type correctionRow struct {
	ID       int64
	ActionID int64
	Payload  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{t: tables{
		Slots: map[int64]string{}, Buffs: map[int64]string{}, Rounds: map[int64]bool{},
		Weapons: map[string]weaponRow{}, Items: map[string]storeq.InsertItemParams{},
		Reviews: map[string]bool{}, Renames: map[string]string{}, SnapshotKeys: map[int64][]string{}, AttackAmmo: map[int64]string{},
		Offers: map[string]bool{}, Profs: map[string]storeq.UpsertProficiencyParams{},
		Bard: map[string]storeq.UpsertBardSkillParams{}, Texts: map[int64]localized.Text{}, ProfDescs: map[string]int64{},
		Names: map[int64]string{}, Races: map[int64]string{}, Alignments: map[int64]string{}, Sexes: map[int64]string{}, Languages: map[int64]string{}, SkillPoints: map[int64]string{}, SkillSets: map[int64]string{}, Picks: map[int64]string{}, Classes: map[int64]string{}, SkillArmors: map[int64]string{}, Spellbook: map[string]storeq.InsertSpellbookEntryParams{},
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
	f.t.Races[id] = arg.RaceCode
	f.t.SkillPoints[id] = "{}"
	f.t.SkillSets[id] = arg.SkillSets
	f.t.Picks[id] = arg.ProficiencyPicks
	f.t.Alignments[id], f.t.Sexes[id] = arg.Alignment, arg.Sex
	f.t.Languages[id] = arg.Languages
	return id, nil
}

func (f *fakeStore) ListCharacterLanguages(context.Context) ([]storeq.ListCharacterLanguagesRow, error) {
	rows := []storeq.ListCharacterLanguagesRow{}
	for _, id := range f.t.Characters {
		rows = append(rows, storeq.ListCharacterLanguagesRow{ID: id, Languages: f.t.Languages[id]})
	}
	return rows, nil
}

func (f *fakeStore) SetCharacterLanguages(_ context.Context, arg storeq.SetCharacterLanguagesParams) error {
	if err := f.hasCharacter(arg.ID); err != nil {
		return err
	}
	f.t.Languages[arg.ID] = arg.Languages
	return nil
}

func (f *fakeStore) ListCharacterIdentity(context.Context) ([]storeq.ListCharacterIdentityRow, error) {
	rows := []storeq.ListCharacterIdentityRow{}
	for _, id := range f.t.Characters {
		rows = append(rows, storeq.ListCharacterIdentityRow{ID: id, Alignment: f.t.Alignments[id], Sex: f.t.Sexes[id]})
	}
	return rows, nil
}

func (f *fakeStore) SetCharacterIdentity(_ context.Context, arg storeq.SetCharacterIdentityParams) error {
	if err := f.hasCharacter(arg.ID); err != nil {
		return err
	}
	f.t.Alignments[arg.ID], f.t.Sexes[arg.ID] = arg.Alignment, arg.Sex
	return nil
}

func (f *fakeStore) SetIdentityIfUnset(_ context.Context, arg storeq.SetIdentityIfUnsetParams) error {
	if !slices.Contains(f.t.Characters, arg.ID) {
		return nil
	}
	if f.t.Alignments[arg.ID] == "" {
		f.t.Alignments[arg.ID] = arg.Alignment
	}
	if f.t.Sexes[arg.ID] == "" {
		f.t.Sexes[arg.ID] = arg.Sex
	}
	return nil
}

func (f *fakeStore) ListLegacySkillPoints(context.Context) ([]storeq.ListLegacySkillPointsRow, error) {
	rows := []storeq.ListLegacySkillPointsRow{}
	for _, id := range f.t.Characters {
		if f.t.SkillPoints[id] != "" && f.t.SkillPoints[id] != "{}" && (f.t.SkillSets[id] == "" || f.t.SkillSets[id] == "{}") {
			rows = append(rows, storeq.ListLegacySkillPointsRow{ID: id, Class: f.class(id), SkillPoints: f.t.SkillPoints[id]})
		}
	}
	return rows, nil
}

func (f *fakeStore) SetSkillSets(_ context.Context, arg storeq.SetSkillSetsParams) error {
	f.t.SkillSets[arg.ID] = arg.SkillSets
	return nil
}

func (f *fakeStore) SetSkillPointsIfUnset(_ context.Context, arg storeq.SetSkillPointsIfUnsetParams) error {
	if slices.Contains(f.t.Characters, arg.ID) && f.t.SkillPoints[arg.ID] == "{}" {
		f.t.SkillPoints[arg.ID] = arg.SkillPoints
	}
	return nil
}

func (f *fakeStore) SetRaceIfUnset(_ context.Context, arg storeq.SetRaceIfUnsetParams) error {
	if slices.Contains(f.t.Characters, arg.ID) && f.t.Races[arg.ID] == "" {
		f.t.Races[arg.ID] = arg.RaceCode
	}
	return nil
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
	return adopt(f, f.t.Weapons, characterID, func(w *weaponRow) *int64 { return &w.CharacterID })
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

// adopt gives every row of table without an owner to characterID (the foreign key holds), under
// its new key; the unique (character_id, id) index of migration 002 refuses a row the character
// already holds.
func adopt[R any](f *fakeStore, table map[string]R, characterID int64, owner func(*R) *int64) error {
	if err := f.hasCharacter(characterID); err != nil {
		return err
	}
	for key, r := range table {
		if o := owner(&r); *o == 0 {
			id := key[len(rowKey(0, "")):]
			if _, taken := table[rowKey(characterID, id)]; taken {
				return fmt.Errorf("duplicate key (%d, %q)", characterID, id)
			}
			*o = characterID
			delete(table, key)
			table[rowKey(characterID, id)] = r
		}
	}
	return nil
}

// rowKey is the primary key of a weapon, item, proficiency or bard skill: (character_id, id).
func rowKey(characterID int64, id string) string { return fmt.Sprintf("%d|%s", characterID, id) }

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

func (f *fakeStore) TypeWeaponIfUntyped(_ context.Context, arg storeq.TypeWeaponIfUntypedParams) error {
	for k, w := range f.t.Weapons {
		if w.ID == arg.ID && w.WeaponType == "" {
			w.WeaponType = arg.WeaponType
			f.t.Weapons[k] = w
		}
	}
	return nil
}

// addWeapon puts a weapons row as a release before migration 016 wrote it, refusing what the
// table's CHECKs and keys refuse.
func (f *fakeStore) addWeapon(arg weaponRow) error {
	if !slices.Contains([]string{"melee", "missile"}, arg.Kind) || (arg.TwoHanded != 0 && arg.TwoHanded != 1) {
		return fmt.Errorf("check violation: weapon %q kind %q two_handed %d", arg.ID, arg.Kind, arg.TwoHanded)
	}
	if arg.CharacterID != 0 {
		if err := f.hasCharacter(arg.CharacterID); err != nil {
			return err
		}
	}
	key := rowKey(arg.CharacterID, arg.ID)
	if _, ok := f.t.Weapons[key]; ok {
		return fmt.Errorf("duplicate key weapons %s", key)
	}
	f.t.Weapons[key] = arg
	return nil
}

func (f *fakeStore) InsertItem(_ context.Context, arg storeq.InsertItemParams) error {
	if !slices.Contains([]string{"stock", "charges", "daily"}, arg.Behaviour) || arg.Count < 0 || arg.MaxCount < 0 {
		return fmt.Errorf("check violation: item %q behaviour %q count %d max %d", arg.ID, arg.Behaviour, arg.Count, arg.MaxCount)
	}
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	key := rowKey(arg.CharacterID, arg.ID)
	if _, ok := f.t.Items[key]; ok {
		return fmt.Errorf("duplicate key items %s", key)
	}
	f.t.Items[key] = arg
	return nil
}

func (f *fakeStore) ItemNameKey(_ context.Context, arg storeq.ItemNameKeyParams) (int64, error) {
	it, ok := f.t.Items[rowKey(arg.CharacterID, arg.ID)]
	if !ok {
		return 0, sql.ErrNoRows
	}
	return it.Name, nil
}

func (f *fakeStore) InsertReview(_ context.Context, arg storeq.InsertReviewParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	f.t.Reviews[fmt.Sprintf("%d|%s|%s", arg.CharacterID, arg.ItemID, arg.Reason)] = true
	return nil
}

func (f *fakeStore) EquipmentOwners(context.Context) ([]int64, error) {
	seen := map[int64]bool{}
	for _, it := range f.t.Items {
		seen[it.CharacterID] = true
	}
	for _, w := range f.t.Weapons {
		seen[w.CharacterID] = true
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
	return out, nil
}

func (f *fakeStore) ListLegacyItems(_ context.Context, characterID int64) ([]storeq.ListLegacyItemsRow, error) {
	var out []storeq.ListLegacyItemsRow
	for _, it := range f.t.Items {
		if it.CharacterID == characterID {
			out = append(out, storeq.ListLegacyItemsRow{
				ID: it.ID, Position: it.Position, Name: it.Name, Type: it.Type, Behaviour: it.Behaviour,
				MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind, Count: it.Count, MaxCount: it.MaxCount,
				EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes, Powers: it.Powers,
			})
		}
	}
	slices.SortFunc(out, func(a, b storeq.ListLegacyItemsRow) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (f *fakeStore) ListLegacyWeapons(_ context.Context, characterID int64) ([]storeq.ListLegacyWeaponsRow, error) {
	var out []storeq.ListLegacyWeaponsRow
	for _, w := range f.t.Weapons {
		if w.CharacterID == characterID {
			out = append(out, storeq.ListLegacyWeaponsRow{
				ID: w.ID, Position: w.Position, Name: w.Name, DamageSm: w.DamageSm, DamageL: w.DamageL,
				MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: w.Kind, TwoHanded: w.TwoHanded,
				AmmoKind: w.AmmoKind, WeaponType: w.WeaponType, Specialized: w.Specialized,
			})
		}
	}
	slices.SortFunc(out, func(a, b storeq.ListLegacyWeaponsRow) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (f *fakeStore) DeleteCharacterItems(_ context.Context, characterID int64) error {
	for k, it := range f.t.Items {
		if it.CharacterID == characterID {
			delete(f.t.Items, k)
		}
	}
	return nil
}

func (f *fakeStore) InsertItemRename(_ context.Context, arg storeq.InsertItemRenameParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	key := rowKey(arg.CharacterID, arg.OldID)
	if _, ok := f.t.Renames[key]; ok {
		return fmt.Errorf("duplicate key item_id_renames %s", key)
	}
	f.t.Renames[key] = arg.NewID
	return nil
}

func (f *fakeStore) RenamedItemID(_ context.Context, arg storeq.RenamedItemIDParams) (string, error) {
	id, ok := f.t.Renames[rowKey(arg.CharacterID, arg.OldID)]
	if !ok {
		return "", sql.ErrNoRows
	}
	return id, nil
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

// The skill upserts conflict on (character_id, id), the primary key: another character's row of
// the same id is another row.
func (f *fakeStore) UpsertProficiency(_ context.Context, arg storeq.UpsertProficiencyParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	f.t.Profs[rowKey(arg.CharacterID, arg.ID)] = arg
	return nil
}

func (f *fakeStore) UpsertBardSkill(_ context.Context, arg storeq.UpsertBardSkillParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	f.t.Bard[rowKey(arg.CharacterID, arg.ID)] = arg
	return nil
}

func spellbookKey(characterID int64, spellKey string) string { return rowKey(characterID, spellKey) }

func (f *fakeStore) CountSpellbookEntries(_ context.Context, characterID int64) (int64, error) {
	return count(f.t.Spellbook, func(e storeq.InsertSpellbookEntryParams) bool { return e.CharacterID == characterID }), nil
}

func (f *fakeStore) InsertSpellbookEntry(_ context.Context, arg storeq.InsertSpellbookEntryParams) error {
	if err := f.hasCharacter(arg.CharacterID); err != nil {
		return err
	}
	key := spellbookKey(arg.CharacterID, arg.SpellKey)
	if _, ok := f.t.Spellbook[key]; ok {
		return fmt.Errorf("duplicate key spellbook_entries %s", key)
	}
	f.t.Spellbook[key] = arg
	return nil
}

func (f *fakeStore) SpellbookRefs(context.Context) ([]storeq.SpellbookRefsRow, error) {
	out := []storeq.SpellbookRefsRow{}
	for _, e := range f.spellbookRows() {
		out = append(out, storeq.SpellbookRefsRow{CharacterID: e.CharacterID, SpellKey: e.SpellKey})
	}
	return out, nil
}

func (f *fakeStore) SpellbookEntryOrigins(context.Context) ([]storeq.SpellbookEntryOriginsRow, error) {
	out := []storeq.SpellbookEntryOriginsRow{}
	for _, e := range f.spellbookRows() {
		out = append(out, storeq.SpellbookEntryOriginsRow{CharacterID: e.CharacterID, SpellKey: e.SpellKey, Origin: e.Origin})
	}
	return out, nil
}

func (f *fakeStore) SetSpellbookEntryOrigin(_ context.Context, arg storeq.SetSpellbookEntryOriginParams) error {
	key := spellbookKey(arg.CharacterID, arg.SpellKey)
	if e, ok := f.t.Spellbook[key]; ok {
		e.Origin = arg.Origin
		f.t.Spellbook[key] = e
	}
	return nil
}

func (f *fakeStore) SpellLearnPayloads(context.Context) ([]storeq.SpellLearnPayloadsRow, error) {
	out := []storeq.SpellLearnPayloadsRow{}
	for _, e := range f.t.Log {
		if e.Kind == "spell_learn" {
			out = append(out, storeq.SpellLearnPayloadsRow{CharacterID: e.CharacterID, Payload: e.Payload})
		}
	}
	return out, nil
}

// spellbookRows is spellbook_entries in the order of the character, then the position.
func (f *fakeStore) spellbookRows() []storeq.InsertSpellbookEntryParams {
	rows := make([]storeq.InsertSpellbookEntryParams, 0, len(f.t.Spellbook))
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
	return rows
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

// nullEnv is an envelope column: "" is NULL (not re-encoded yet).
func nullEnv(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func (f *fakeStore) SnapshotsOf(_ context.Context, stateKey string) ([]storeq.SnapshotsOfRow, error) {
	out := []storeq.SnapshotsOfRow{}
	for _, s := range f.t.Snapshots {
		if s.Key == stateKey {
			out = append(out, storeq.SnapshotsOfRow{ID: s.ID, BeforeEnvelope: nullEnv(s.BeforeEnv), AfterEnvelope: nullEnv(s.AfterEnv)})
		}
	}
	return out, nil
}

func (f *fakeStore) SnapshotsWithPrefix(_ context.Context, prefix string) ([]storeq.SnapshotsWithPrefixRow, error) {
	out := []storeq.SnapshotsWithPrefixRow{}
	for _, s := range f.t.Snapshots {
		if strings.HasPrefix(s.Key, prefix) {
			out = append(out, storeq.SnapshotsWithPrefixRow{
				ID: s.ID, StateKey: s.Key, BeforeEnvelope: nullEnv(s.BeforeEnv), AfterEnvelope: nullEnv(s.AfterEnv),
			})
		}
	}
	return out, nil
}

func (f *fakeStore) SnapshotsToReencode(context.Context) ([]storeq.SnapshotsToReencodeRow, error) {
	out := []storeq.SnapshotsToReencodeRow{}
	for _, s := range f.t.Snapshots {
		if s.BeforeEnv == "" || s.AfterEnv == "" {
			out = append(out, storeq.SnapshotsToReencodeRow{
				ID: s.ID, CharacterID: s.CharacterID, StateKey: s.Key, BeforeState: s.Before, AfterState: s.After,
			})
		}
	}
	return out, nil
}

func (f *fakeStore) SetSnapshotEnvelopes(_ context.Context, arg storeq.SetSnapshotEnvelopesParams) error {
	for i := range f.t.Snapshots {
		if f.t.Snapshots[i].ID == arg.ID {
			f.t.Snapshots[i].BeforeEnv, f.t.Snapshots[i].AfterEnv = arg.BeforeEnvelope, arg.AfterEnvelope
		}
	}
	return nil
}

func (f *fakeStore) LearnFailureRefs(context.Context) ([]storeq.LearnFailureRefsRow, error) {
	return append([]storeq.LearnFailureRefsRow{}, f.t.LearnFailures...), nil
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

func (f *fakeStore) LegacySnapshotKeys(_ context.Context, characterID int64) ([]string, error) {
	return slices.Clone(f.t.SnapshotKeys[characterID]), nil
}

func (f *fakeStore) RenameAttackFormAmmo(_ context.Context, arg storeq.RenameAttackFormAmmoParams) error {
	if f.t.AttackAmmo[arg.CharacterID] == arg.OldID {
		f.t.AttackAmmo[arg.CharacterID] = arg.NewID
	}
	return nil
}

func (f *fakeStore) SkillArmourCharacters(context.Context) ([]storeq.SkillArmourCharactersRow, error) {
	rows := []storeq.SkillArmourCharactersRow{}
	for _, id := range f.t.Characters {
		rows = append(rows, storeq.SkillArmourCharactersRow{ID: id, Class: f.class(id), SkillArmor: f.t.SkillArmors[id]})
	}
	return rows, nil
}

func (f *fakeStore) CharacterSkillArmour(_ context.Context, id int64) (storeq.CharacterSkillArmourRow, error) {
	if !slices.Contains(f.t.Characters, id) {
		return storeq.CharacterSkillArmourRow{}, sql.ErrNoRows
	}
	return storeq.CharacterSkillArmourRow{Class: f.class(id), SkillArmor: f.t.SkillArmors[id]}, nil
}

// ProficiencyRows are the proficiencies rows by character, then position, then id.
func (f *fakeStore) ProficiencyRows(context.Context) ([]storeq.ProficiencyRowsRow, error) {
	all := make([]storeq.UpsertProficiencyParams, 0, len(f.t.Profs))
	for _, p := range f.t.Profs {
		all = append(all, p)
	}
	slices.SortFunc(all, func(a, b storeq.UpsertProficiencyParams) int {
		return cmp.Or(cmp.Compare(a.CharacterID, b.CharacterID), cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	rows := make([]storeq.ProficiencyRowsRow, 0, len(all))
	for _, p := range all {
		rows = append(rows, storeq.ProficiencyRowsRow{CharacterID: p.CharacterID, ID: p.ID})
	}
	return rows, nil
}

func (f *fakeStore) CharacterClasses(context.Context) ([]storeq.CharacterClassesRow, error) {
	rows := []storeq.CharacterClassesRow{}
	for _, id := range f.t.Characters {
		rows = append(rows, storeq.CharacterClassesRow{ID: id, Class: f.class(id)})
	}
	return rows, nil
}

func (f *fakeStore) SetCharacterPicks(_ context.Context, arg storeq.SetCharacterPicksParams) error {
	if slices.Contains(f.t.Characters, arg.ID) {
		f.t.Picks[arg.ID] = arg.ProficiencyPicks
	}
	return nil
}

func (f *fakeStore) CharacterWeaponTypes(context.Context) ([]storeq.CharacterWeaponTypesRow, error) {
	seen := map[storeq.CharacterWeaponTypesRow]bool{}
	rows := []storeq.CharacterWeaponTypesRow{}
	for _, it := range f.t.Items {
		r := storeq.CharacterWeaponTypesRow{CharacterID: it.CharacterID, WeaponType: it.WeaponType}
		if it.Category == "weapon" && it.WeaponType != "" && !seen[r] {
			seen[r] = true
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b storeq.CharacterWeaponTypesRow) int {
		if a.CharacterID != b.CharacterID {
			return int(a.CharacterID - b.CharacterID)
		}
		return strings.Compare(a.WeaponType, b.WeaponType)
	})
	return rows, nil
}

func (f *fakeStore) SpecializedWeaponTypes(context.Context) ([]storeq.SpecializedWeaponTypesRow, error) {
	items := []storeq.InsertItemParams{}
	for _, it := range f.t.Items {
		if it.Category == "weapon" && it.Specialized == 1 && it.WeaponType != "" {
			items = append(items, it)
		}
	}
	slices.SortFunc(items, func(a, b storeq.InsertItemParams) int {
		if a.CharacterID != b.CharacterID {
			return int(a.CharacterID - b.CharacterID)
		}
		if a.Position != b.Position {
			return int(a.Position - b.Position)
		}
		return strings.Compare(a.ID, b.ID)
	})
	rows := make([]storeq.SpecializedWeaponTypesRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, storeq.SpecializedWeaponTypesRow{CharacterID: it.CharacterID, WeaponType: it.WeaponType})
	}
	return rows, nil
}

func (f *fakeStore) CharacterPicks(context.Context) ([]storeq.CharacterPicksRow, error) {
	rows := []storeq.CharacterPicksRow{}
	for _, id := range f.t.Characters {
		rows = append(rows, storeq.CharacterPicksRow{ID: id, ProficiencyPicks: f.picks(id)})
	}
	return rows, nil
}

// picks is characters.proficiency_picks, '{}' (the column's default) when none was set.
func (f *fakeStore) picks(id int64) string {
	if p, ok := f.t.Picks[id]; ok {
		return p
	}
	return "{}"
}

// class is characters.class, 'bard' (the column's default) when none was set.
func (f *fakeStore) class(id int64) string {
	if c, ok := f.t.Classes[id]; ok {
		return c
	}
	return "bard"
}

func (f *fakeStore) CarryOverItems(_ context.Context, characterID int64) ([]storeq.CarryOverItemsRow, error) {
	rows := []storeq.CarryOverItemsRow{}
	for _, it := range f.t.Items {
		if it.CharacterID == characterID {
			rows = append(rows, storeq.CarryOverItemsRow{
				ID: it.ID, Position: it.Position, Category: it.Category, State: it.State, SkillColumn: it.SkillColumn,
			})
		}
	}
	slices.SortFunc(rows, func(a, b storeq.CarryOverItemsRow) int {
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	return rows, nil
}

func (f *fakeStore) WearItem(_ context.Context, arg storeq.WearItemParams) error {
	key := rowKey(arg.CharacterID, arg.ID)
	it, ok := f.t.Items[key]
	if !ok {
		return nil
	}
	it.State = "worn"
	f.t.Items[key] = it
	return nil
}

func (f *fakeStore) ProficiencyDescriptionKeys(context.Context) ([]storeq.ProficiencyDescription, error) {
	out := []storeq.ProficiencyDescription{}
	for id, key := range f.t.ProfDescs {
		out = append(out, storeq.ProficiencyDescription{ProficiencyID: id, Description: key})
	}
	return out, nil
}

func (f *fakeStore) UpsertProficiencyDescription(_ context.Context, arg storeq.UpsertProficiencyDescriptionParams) error {
	f.t.ProfDescs[arg.ProficiencyID] = arg.Description
	return nil
}

func (f *fakeStore) DeleteProficiencyDescription(_ context.Context, id string) error {
	delete(f.t.ProfDescs, id)
	return nil
}

func (f *fakeStore) ProficiencyDescriptionTexts(context.Context) ([]storeq.ProficiencyDescriptionTextsRow, error) {
	out := []storeq.ProficiencyDescriptionTextsRow{}
	for id, key := range f.t.ProfDescs {
		for _, lang := range f.t.Texts[key].Languages() {
			out = append(out, storeq.ProficiencyDescriptionTextsRow{ProficiencyID: id, Lang: lang, Body: f.t.Texts[key][lang]})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProficiencyID != out[j].ProficiencyID {
			return out[i].ProficiencyID < out[j].ProficiencyID
		}
		return out[i].Lang < out[j].Lang
	})
	return out, nil
}

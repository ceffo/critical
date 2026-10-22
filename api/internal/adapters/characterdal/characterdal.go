// Package characterdal is the storage adapter for charactersvc and
// detailssvc. It delegates to sqlc-generated queries in the queries/
// subpackage.
package characterdal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// DAL is the implementation of charactersvc.Reader, charactersvc.Owners and
// detailssvc.DAL over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

var (
	_ charactersvc.Reader = (*DAL)(nil)
	_ charactersvc.Owners = (*DAL)(nil)
	_ detailssvc.DAL      = (*DAL)(nil)
	_ acsvc.Overrides     = (*DAL)(nil)
)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// GetCharacter reads the row by id and maps it to a domain Character.
// Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error) {
	row, err := d.q.GetCharacter(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	langs, err := decodeLanguages(row.Languages)
	if err != nil {
		return nil, err
	}
	points, err := decodeSkillPoints(row.SkillSets, row.SkillPoints)
	if err != nil {
		return nil, err
	}
	picks, err := DecodePicks(row.ProficiencyPicks)
	if err != nil {
		return nil, err
	}
	entries, err := d.classEntries(ctx, row)
	if err != nil {
		return nil, err
	}
	c := &charactersvc.Character{
		ID: row.ID, Name: row.Name,
		Str: int(row.Str), StrExceptional: int(row.StrExceptional), Dex: int(row.Dex), Con: int(row.Con),
		Int: int(row.Int), Wis: int(row.Wis), Cha: int(row.Cha),
		HPMax: int(row.HpMax), Race: row.RaceCode, SkillPoints: points, Proficiencies: picks,
		// The figures the imported sheet stated, kept as imported (ADR 0030 step 24, retired).
		Sheet: charactersvc.Sheet{Level: int(row.Level), THAC0: int(row.Thac0Base), Saves: charactersvc.Saves{
			ParaPoison: int(row.SaveParaPoison), RodStaffWand: int(row.SaveRodStaffWand),
			PetriPoly: int(row.SavePetriPoly), Breath: int(row.SaveBreath), Spell: int(row.SaveSpell),
		}},
		Details: charactersvc.Details{
			Alignment: row.Alignment, Age: int(row.Age), Sex: row.Sex,
			Height: row.HeightCm, Weight: row.WeightKg, Hair: row.Hair, Eyes: row.Eyes, Languages: langs,
		},
	}
	c.SetClasses(entries)
	return c, nil
}

// classEntries are the character's class entries (ADR 0042 step 1). A character with none stored
// (created by the release before migration 035 during a rolling deploy) has the one active entry
// its columns state, as the migration's data step gives every stored character.
func (d *DAL) classEntries(ctx context.Context, row queries.GetCharacterRow) ([]classes.Entry, error) {
	rows, err := d.q.ListClassEntries(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("list class entries of character %d: %w", row.ID, err)
	}
	if len(rows) == 0 {
		return []classes.Entry{classes.SingleEntry(row.Class, int(row.RealizedLevel), int(row.XpTotal))}, nil
	}
	out := make([]classes.Entry, len(rows))
	for i, r := range rows {
		out[i] = classes.Entry{
			Class: r.Class, RealizedLevel: int(r.RealizedLevel), StartingXP: int(r.StartingXp),
			State: classes.EntryState(r.State), LeftAtLevel: int(r.LeftAtLevel),
			IgnoredAwardsThrough: r.IgnoredAwardsThrough,
		}
	}
	return out, nil
}

// SetRealizedLevel implements levelupsvc.DAL: the realized level of the class
// entry of class ("" the first active one) and the max HP that goes with it
// (ADR 0030 step 7.3, ADR 0042 step 7). domain.ErrCharacterNotFound when the
// row (or the active entry) is missing.
func (d *DAL) SetRealizedLevel(ctx context.Context, id int64, class string, realizedLevel, hpMax int) error {
	entries, err := d.q.ListClassEntries(ctx, id)
	if err != nil {
		return fmt.Errorf("list class entries of character %d: %w", id, err)
	}
	if class != "" && !firstActiveIs(entries, class) {
		return d.setOtherEntryLevel(ctx, id, class, realizedLevel, hpMax)
	}
	n, err := d.q.SetRealizedLevel(ctx, queries.SetRealizedLevelParams{
		RealizedLevel: sql.NullInt64{Int64: int64(realizedLevel), Valid: true}, HpMax: int64(hpMax), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set realized level: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	// The first active entry takes the level too (the column above is the release before the class
	// entries' reading of it; a character with no entry has only the column).
	if err := d.q.SetFirstActiveEntryLevel(ctx, queries.SetFirstActiveEntryLevelParams{
		RealizedLevel: int64(realizedLevel), CharacterID: id,
	}); err != nil {
		return fmt.Errorf("set realized level of the class entry: %w", err)
	}
	return nil
}

// ReplaceClassEntries implements dualclasssvc.DAL: the character's class entries are replaced
// whole (ADR 0042 step 3, and the undo and redo of taking a class). The characters columns keep
// the first active entry's class, level and starting XP (migration 035). Refused as
// classes.ValidateEntries does; domain.ErrCharacterNotFound for an unknown character.
func (d *DAL) ReplaceClassEntries(ctx context.Context, id int64, entries []classes.Entry) error {
	if err := classes.ValidateEntries(entries); err != nil {
		return err
	}
	first, _ := classes.Primary(entries) // ValidateEntries refused a character with no active entry
	n, err := d.q.SetClassColumns(ctx, queries.SetClassColumnsParams{
		Class: first.Class, RealizedLevel: sql.NullInt64{Int64: int64(first.RealizedLevel), Valid: true},
		XpTotal: int64(first.StartingXP), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set class columns: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	if err := d.q.DeleteClassEntries(ctx, id); err != nil {
		return fmt.Errorf("delete class entries of character %d: %w", id, err)
	}
	for i, e := range entries {
		if err := d.q.InsertClassEntry(ctx, queries.InsertClassEntryParams{
			CharacterID: id, Position: int64(i), Class: e.Class, RealizedLevel: int64(e.RealizedLevel),
			StartingXp: int64(e.StartingXP), State: string(e.State), LeftAtLevel: int64(e.LeftAtLevel),
			IgnoredAwardsThrough: e.IgnoredAwardsThrough,
		}); err != nil {
			return fmt.Errorf("insert class entry %d of character %d: %w", i, id, err)
		}
	}
	return nil
}

// firstActiveIs reports whether class is the first active entry's, or there
// is no entry (the columns are the one entry).
func firstActiveIs(entries []queries.ListClassEntriesRow, class string) bool {
	for _, e := range entries {
		if e.State == string(classes.EntryActive) {
			return e.Class == class
		}
	}
	return true
}

// setOtherEntryLevel is a level-up of an entry other than the first active
// one: its level and the max HP; the columns keep the first's level.
func (d *DAL) setOtherEntryLevel(ctx context.Context, id int64, class string, realizedLevel, hpMax int) error {
	n, err := d.q.SetHPMax(ctx, queries.SetHPMaxParams{HpMax: int64(hpMax), ID: id})
	if err != nil {
		return fmt.Errorf("set max HP: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	if n, err = d.q.SetEntryLevel(ctx, queries.SetEntryLevelParams{
		RealizedLevel: int64(realizedLevel), CharacterID: id, Class: class,
	}); err != nil {
		return fmt.Errorf("set realized level of class entry %s: %w", class, err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// SetSkillPoints implements skillsvc.Points: a character's points per skill set,
// nil for none (ADR 0030 step 17). domain.ErrCharacterNotFound when the row
// is missing.
func (d *DAL) SetSkillPoints(ctx context.Context, id int64, points skills.SetPoints) error {
	raw, err := encodeSkillPoints(points)
	if err != nil {
		return err
	}
	n, err := d.q.SetSkillPoints(ctx, queries.SetSkillPointsParams{SkillSets: raw, ID: id})
	if err != nil {
		return fmt.Errorf("set skill points: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// SkillPoints implements skillsvc.Points: the class and the stored points of every skill set.
// domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) SkillPoints(ctx context.Context, id int64) (string, skills.SetPoints, error) {
	row, err := d.q.GetSkillSets(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, domain.ErrCharacterNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("get skill points: %w", err)
	}
	points, err := decodeSkillPoints(row.SkillSets, row.SkillPoints)
	if err != nil {
		return "", nil, err
	}
	return row.Class, points, nil
}

// SetLanguages implements skillsvc.Picks: the character's languages (ADR 0032 step 4, saved by
// the Skills tab, ADR 0037 step 18). domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) SetLanguages(ctx context.Context, id int64, languages []string) error {
	langs, err := json.Marshal(append([]string{}, languages...))
	if err != nil {
		return fmt.Errorf("encode languages: %w", err)
	}
	n, err := d.q.SetLanguages(ctx, queries.SetLanguagesParams{Languages: string(langs), ID: id})
	if err != nil {
		return fmt.Errorf("set languages: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// SetProficiencyPicks implements skillsvc.Picks: the character's proficiency picks (ADR 0037
// step 12). domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) SetProficiencyPicks(ctx context.Context, id int64, picks skills.Picks) error {
	raw, err := EncodePicks(picks)
	if err != nil {
		return err
	}
	n, err := d.q.SetProficiencyPicks(ctx, queries.SetProficiencyPicksParams{ProficiencyPicks: raw, ID: id})
	if err != nil {
		return fmt.Errorf("set proficiency picks: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// GetACOverride implements acsvc.Overrides: nil for none (ADR 0033 step
// 13). domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetACOverride(ctx context.Context, id int64) (*acsvc.Override, error) {
	row, err := d.q.GetACOverride(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ac override: %w", err)
	}
	if !row.AcOverride.Valid {
		return nil, nil
	}
	return &acsvc.Override{Value: int(row.AcOverride.Int32), Label: row.AcOverrideLabel}, nil
}

// SetACOverride implements acsvc.Overrides: nil clears it.
// domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) SetACOverride(ctx context.Context, id int64, o *acsvc.Override) error {
	p := queries.SetACOverrideParams{ID: id}
	if o != nil {
		p.AcOverride = sql.NullInt32{Int32: int32(o.Value), Valid: true} //nolint:gosec // -10..10, checked by acsvc
		p.AcOverrideLabel = o.Label
	}
	n, err := d.q.SetACOverride(ctx, p)
	if err != nil {
		return fmt.Errorf("set ac override: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// CharactersOf implements charactersvc.Owners.
func (d *DAL) CharactersOf(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error) {
	rows, err := d.q.CharactersOf(ctx, uuid.NullUUID{UUID: owner, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("characters of %s: %w", owner, err)
	}
	out := make([]charactersvc.Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, charactersvc.Summary{ID: r.ID, Name: r.Name})
	}
	return out, nil
}

// SetCurrentCharacter implements charactersvc.Owners: the update matches only
// a character the user owns and has not retired.
func (d *DAL) SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error {
	if err := d.lockUser(ctx, owner); err != nil {
		return err
	}
	n, err := d.q.SetCurrentCharacter(ctx, queries.SetCurrentCharacterParams{
		CharacterID: sql.NullInt64{Int64: id, Valid: true}, UserID: owner,
	})
	if err != nil {
		return fmt.Errorf("set current character: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// CreateCharacter implements charactersvc.Owners: the character's row, then
// the rows that refer to it (its spell slots, buffs and round state). The
// sheet's figures are c.Sheet (the derived values, kept as stored); the
// details start empty.
func (d *DAL) CreateCharacter(ctx context.Context, owner uuid.UUID, c charactersvc.Character) (int64, error) {
	if err := d.lockUser(ctx, owner); err != nil {
		return 0, err
	}
	points, err := encodeSkillPoints(c.SkillPoints)
	if err != nil {
		return 0, err
	}
	picks, err := EncodePicks(c.Proficiencies)
	if err != nil {
		return 0, err
	}
	entries := c.Entries()
	if err := classes.ValidateEntries(entries); err != nil {
		return 0, err
	}
	first, _ := classes.Primary(entries) // ValidateEntries refused a character with no active entry
	id, err := d.q.CreateCharacter(ctx, queries.CreateCharacterParams{
		Name: c.Name, Str: int64(c.Str), StrExceptional: int64(c.StrExceptional), Dex: int64(c.Dex), Con: int64(c.Con), Int: int64(c.Int),
		Wis: int64(c.Wis), Cha: int64(c.Cha), HpMax: int64(c.HPMax), XpTotal: int64(first.StartingXP),
		Thac0Base: int64(c.Sheet.THAC0), Level: int64(c.Sheet.Level),
		SaveParaPoison: int64(c.Sheet.Saves.ParaPoison), SaveRodStaffWand: int64(c.Sheet.Saves.RodStaffWand),
		SavePetriPoly: int64(c.Sheet.Saves.PetriPoly), SaveBreath: int64(c.Sheet.Saves.Breath),
		SaveSpell: int64(c.Sheet.Saves.Spell), Class: first.Class, RaceCode: c.Race,
		OwnerUserID:   uuid.NullUUID{UUID: owner, Valid: true},
		RealizedLevel: sql.NullInt64{Int64: int64(first.RealizedLevel), Valid: true},
		SkillSets:     points, ProficiencyPicks: picks,
	})
	if err != nil {
		return 0, fmt.Errorf("create character: %w", err)
	}
	for i, e := range entries {
		if err := d.q.InsertClassEntry(ctx, queries.InsertClassEntryParams{
			CharacterID: id, Position: int64(i), Class: e.Class, RealizedLevel: int64(e.RealizedLevel),
			StartingXp: int64(e.StartingXP), State: string(e.State), LeftAtLevel: int64(e.LeftAtLevel),
			IgnoredAwardsThrough: e.IgnoredAwardsThrough,
		}); err != nil {
			return 0, fmt.Errorf("create character %d class entry %d: %w", id, i, err)
		}
	}
	for _, row := range []struct {
		what   string
		insert func(context.Context, int64) error
	}{
		{"spell slots", d.q.CreateSpellSlots}, {"buffs", d.q.CreateActiveBuffs}, {"round state", d.q.CreateRoundState},
	} {
		if err := row.insert(ctx, id); err != nil {
			return 0, fmt.Errorf("create character %d %s: %w", id, row.what, err)
		}
	}
	// The details and the languages entered with the character, in the same transaction.
	if err := d.UpdateDetails(ctx, id, c.Details); err != nil {
		return 0, fmt.Errorf("create character %d: %w", id, err)
	}
	if err := d.SetLanguages(ctx, id, c.Details.Languages); err != nil {
		return 0, fmt.Errorf("create character %d: %w", id, err)
	}
	return id, nil
}

// RenameCharacter implements charactersvc.Owners.
func (d *DAL) RenameCharacter(ctx context.Context, owner uuid.UUID, id int64, name string) error {
	n, err := d.q.RenameCharacter(ctx, queries.RenameCharacterParams{
		Name: name, ID: id, OwnerUserID: uuid.NullUUID{UUID: owner, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("rename character: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// RestoreSkillArmor implements charactersvc.Owners.
func (d *DAL) RestoreSkillArmor(ctx context.Context, id int64, armor string) error {
	n, err := d.q.RestoreSkillArmor(ctx, queries.RestoreSkillArmorParams{SkillArmor: armor, ID: id})
	if err != nil {
		return fmt.Errorf("restore skill armor: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// RetireCharacter implements charactersvc.Owners.
func (d *DAL) RetireCharacter(ctx context.Context, owner uuid.UUID, id int64) error {
	if err := d.lockUser(ctx, owner); err != nil {
		return err
	}
	n, err := d.q.RetireCharacter(ctx, queries.RetireCharacterParams{
		ID: id, OwnerUserID: uuid.NullUUID{UUID: owner, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("retire character: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// RepairCurrentCharacter implements charactersvc.Owners.
func (d *DAL) RepairCurrentCharacter(ctx context.Context, owner uuid.UUID) error {
	if err := d.q.RepairCurrentCharacter(ctx, owner); err != nil {
		return fmt.Errorf("repair current character: %w", err)
	}
	return nil
}

// lockUser takes the user's row lock until the transaction ends, so that a
// switch, a creation and a retirement of one user run one after the other.
// An unknown user is no lock to take: the guarded update then matches nothing.
func (d *DAL) lockUser(ctx context.Context, owner uuid.UUID) error {
	if _, err := d.q.LockUser(ctx, owner); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock user %s: %w", owner, err)
	}
	return nil
}

// CurrentCharacter implements charactersvc.Owners.
func (d *DAL) CurrentCharacter(ctx context.Context, owner uuid.UUID) (int64, error) {
	id, err := d.q.CurrentCharacter(ctx, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("current character: %w", err)
	}
	return id, nil
}

// decodeSkillPoints reads characters.skill_sets (a JSON object from skill set code to an object
// from skill code to points); '{}' is none (nil). While it is '{}' the legacy flat
// characters.skill_points still counts: a release from before skill sets, running during a
// rolling deploy, keeps writing only that column. Its points come back under skills.LegacySet,
// for the services to file under the class's first set (SetPoints.Resolve).
func decodeSkillPoints(raw, legacy string) (skills.SetPoints, error) {
	var out skills.SetPoints
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode skill points: %w", err)
	}
	if len(out) > 0 {
		return out, nil
	}
	if legacy == "" {
		return nil, nil
	}
	var flat map[string]int
	if err := json.Unmarshal([]byte(legacy), &flat); err != nil {
		return nil, fmt.Errorf("decode legacy skill points: %w", err)
	}
	if len(flat) == 0 {
		return nil, nil
	}
	return skills.SetPoints{skills.LegacySet: flat}, nil
}

// encodeSkillPoints writes characters.skill_sets; nil is '{}'.
func encodeSkillPoints(points skills.SetPoints) (string, error) {
	if points == nil {
		points = skills.SetPoints{}
	}
	raw, err := json.Marshal(points)
	if err != nil {
		return "", fmt.Errorf("encode skill points: %w", err)
	}
	return string(raw), nil
}

func decodeLanguages(raw string) ([]string, error) {
	langs := []string{}
	if err := json.Unmarshal([]byte(raw), &langs); err != nil {
		return nil, fmt.Errorf("decode languages: %w", err)
	}
	return langs, nil
}

// GetDetails reads the editable details and the character's class and race
// codes. Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetDetails(ctx context.Context, id int64) (charactersvc.Details, charactersvc.Kind, error) {
	r, err := d.q.GetCharacterDetails(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return charactersvc.Details{}, charactersvc.Kind{}, domain.ErrCharacterNotFound
	}
	if err != nil {
		return charactersvc.Details{}, charactersvc.Kind{}, fmt.Errorf("get character details: %w", err)
	}
	langs, err := decodeLanguages(r.Languages)
	if err != nil {
		return charactersvc.Details{}, charactersvc.Kind{}, err
	}
	return charactersvc.Details{
		Alignment: r.Alignment, Age: int(r.Age), Sex: r.Sex,
		Height: r.HeightCm, Weight: r.WeightKg, Hair: r.Hair, Eyes: r.Eyes, Languages: langs,
	}, charactersvc.Kind{Class: r.Class, Race: r.RaceCode}, nil
}

// UpdateDetails overwrites every editable detail of the character but its languages, which
// SetLanguages writes (ADR 0037 step 18).
func (d *DAL) UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error {
	n, err := d.q.UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{
		Alignment: v.Alignment, Age: int64(v.Age), Sex: v.Sex, HeightCm: v.Height,
		WeightKg: v.Weight, Hair: v.Hair, Eyes: v.Eyes, ID: id,
	})
	if err != nil {
		return fmt.Errorf("update character details: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

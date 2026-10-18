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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
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
	points, err := decodeSkillPoints(row.SkillPoints)
	if err != nil {
		return nil, err
	}
	return &charactersvc.Character{
		ID: row.ID, Name: row.Name,
		Str: int(row.Str), StrExceptional: int(row.StrExceptional), Dex: int(row.Dex), Con: int(row.Con),
		Int: int(row.Int), Wis: int(row.Wis), Cha: int(row.Cha),
		HPMax: int(row.HpMax), RealizedLevel: int(row.RealizedLevel), XPTotal: int(row.XpTotal),
		Class: row.Class, Race: row.RaceCode, SkillPoints: points, SkillArmor: row.SkillArmor,
		// The figures the imported sheet stated (ADR 0030 step 24).
		Sheet: charactersvc.Sheet{Level: int(row.Level), THAC0: int(row.Thac0Base), Saves: charactersvc.Saves{
			ParaPoison: int(row.SaveParaPoison), RodStaffWand: int(row.SaveRodStaffWand),
			PetriPoly: int(row.SavePetriPoly), Breath: int(row.SaveBreath), Spell: int(row.SaveSpell),
		}},
		Details: charactersvc.Details{
			Race: row.Race, Alignment: row.Alignment, Age: int(row.Age), Sex: row.Sex,
			Height: row.Height, Weight: row.Weight, Hair: row.Hair, Eyes: row.Eyes, Languages: langs,
		},
	}, nil
}

// SetRealizedLevel implements levelupsvc.DAL: the realized level and the
// max HP that goes with it (ADR 0030 step 7.3). domain.ErrCharacterNotFound
// when the row is missing.
func (d *DAL) SetRealizedLevel(ctx context.Context, id int64, realizedLevel, hpMax int) error {
	n, err := d.q.SetRealizedLevel(ctx, queries.SetRealizedLevelParams{
		RealizedLevel: sql.NullInt64{Int64: int64(realizedLevel), Valid: true}, HpMax: int64(hpMax), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set realized level: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// SetSkillPoints implements skillsvc.Points: a thief's points per skill,
// nil for none (ADR 0030 step 17). domain.ErrCharacterNotFound when the row
// is missing.
func (d *DAL) SetSkillPoints(ctx context.Context, id int64, points map[string]int) error {
	raw, err := encodeSkillPoints(points)
	if err != nil {
		return err
	}
	n, err := d.q.SetSkillPoints(ctx, queries.SetSkillPointsParams{SkillPoints: raw, ID: id})
	if err != nil {
		return fmt.Errorf("set skill points: %w", err)
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
		out = append(out, charactersvc.Summary{ID: r.ID, Name: r.Name, Class: r.Class})
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
// sheet's figures are c.Sheet (the derived values, ADR 0030 step 24); the
// details start empty.
func (d *DAL) CreateCharacter(ctx context.Context, owner uuid.UUID, c charactersvc.Character) (int64, error) {
	if err := d.lockUser(ctx, owner); err != nil {
		return 0, err
	}
	points, err := encodeSkillPoints(c.SkillPoints)
	if err != nil {
		return 0, err
	}
	id, err := d.q.CreateCharacter(ctx, queries.CreateCharacterParams{
		Name: c.Name, Str: int64(c.Str), StrExceptional: int64(c.StrExceptional), Dex: int64(c.Dex), Con: int64(c.Con), Int: int64(c.Int),
		Wis: int64(c.Wis), Cha: int64(c.Cha), HpMax: int64(c.HPMax), XpTotal: int64(c.XPTotal),
		Thac0Base: int64(c.Sheet.THAC0), Level: int64(c.Sheet.Level),
		SaveParaPoison: int64(c.Sheet.Saves.ParaPoison), SaveRodStaffWand: int64(c.Sheet.Saves.RodStaffWand),
		SavePetriPoly: int64(c.Sheet.Saves.PetriPoly), SaveBreath: int64(c.Sheet.Saves.Breath),
		SaveSpell: int64(c.Sheet.Saves.Spell), Class: c.Class, RaceCode: c.Race,
		OwnerUserID:   uuid.NullUUID{UUID: owner, Valid: true},
		RealizedLevel: sql.NullInt64{Int64: int64(c.RealizedLevel), Valid: true},
		SkillPoints:   points, SkillArmor: c.SkillArmor,
	})
	if err != nil {
		return 0, fmt.Errorf("create character: %w", err)
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
	// The details entered with the character, in the same transaction.
	if err := d.UpdateDetails(ctx, id, c.Details); err != nil {
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

// decodeSkillPoints reads characters.skill_points (a JSON object from skill
// code to points); '{}' is none (nil).
func decodeSkillPoints(raw string) (map[string]int, error) {
	var out map[string]int
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode skill points: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// encodeSkillPoints writes characters.skill_points; nil is '{}'.
func encodeSkillPoints(points map[string]int) (string, error) {
	if points == nil {
		points = map[string]int{}
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

// GetDetails reads the editable details. Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetDetails(ctx context.Context, id int64) (charactersvc.Details, error) {
	r, err := d.q.GetCharacterDetails(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return charactersvc.Details{}, domain.ErrCharacterNotFound
	}
	if err != nil {
		return charactersvc.Details{}, fmt.Errorf("get character details: %w", err)
	}
	langs, err := decodeLanguages(r.Languages)
	if err != nil {
		return charactersvc.Details{}, err
	}
	return charactersvc.Details{
		Race: r.Race, Alignment: r.Alignment, Age: int(r.Age), Sex: r.Sex,
		Height: r.Height, Weight: r.Weight, Hair: r.Hair, Eyes: r.Eyes, Languages: langs,
	}, nil
}

// UpdateDetails overwrites every editable detail of the character.
func (d *DAL) UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error {
	langs, err := json.Marshal(append([]string{}, v.Languages...))
	if err != nil {
		return fmt.Errorf("encode languages: %w", err)
	}
	n, err := d.q.UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{
		Race: v.Race, Alignment: v.Alignment, Age: int64(v.Age), Sex: v.Sex, Height: v.Height,
		Weight: v.Weight, Hair: v.Hair, Eyes: v.Eyes, Languages: string(langs), ID: id,
	})
	if err != nil {
		return fmt.Errorf("update character details: %w", err)
	}
	if n == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

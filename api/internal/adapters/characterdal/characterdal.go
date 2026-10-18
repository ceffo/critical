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
	return &charactersvc.Character{
		ID: row.ID, Name: row.Name,
		Str: int(row.Str), Dex: int(row.Dex), Con: int(row.Con),
		Int: int(row.Int), Wis: int(row.Wis), Cha: int(row.Cha),
		HPMax: int(row.HpMax), Thac0Base: int(row.Thac0Base),
		Level: int(row.Level), XPTotal: int(row.XpTotal),
		SaveParaPoison:   int(row.SaveParaPoison),
		SaveRodStaffWand: int(row.SaveRodStaffWand),
		SavePetriPoly:    int(row.SavePetriPoly),
		SaveBreath:       int(row.SaveBreath),
		SaveSpell:        int(row.SaveSpell),
		Class:            row.Class,
		Details: charactersvc.Details{
			Race: row.Race, Alignment: row.Alignment, Age: int(row.Age), Sex: row.Sex,
			Height: row.Height, Weight: row.Weight, Hair: row.Hair, Eyes: row.Eyes, Languages: langs,
		},
	}, nil
}

// CharactersOf implements charactersvc.Owners.
func (d *DAL) CharactersOf(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error) {
	rows, err := d.q.CharactersOf(ctx, uuid.NullUUID{UUID: owner, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("characters of %s: %w", owner, err)
	}
	out := make([]charactersvc.Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, charactersvc.Summary{ID: r.ID, Name: r.Name, Class: r.Class, Level: int(r.Level)})
	}
	return out, nil
}

// SetCurrentCharacter implements charactersvc.Owners: the update matches only
// a character the user owns and has not retired.
func (d *DAL) SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error {
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

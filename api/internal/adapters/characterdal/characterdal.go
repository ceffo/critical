// Package characterdal is the storage adapter for charactersvc. It delegates
// to bob-generated typed queries in the queries/ subpackage.
package characterdal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of charactersvc.Reader.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to the Querier.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec() sqlite.BobExec { return sqlite.BobExec{Querier: d.q} }

// GetCharacter reads the row by id and maps it to a domain Character.
// Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error) {
	row, err := queries.GetCharacter(id).One(ctx, d.exec())
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
		Str: int(row.STR), Dex: int(row.Dex), Con: int(row.Con),
		Int: int(row.Int), Wis: int(row.Wis), Cha: int(row.Cha),
		HPMax: int(row.HPMax), Thac0Base: int(row.Thac0Base),
		Level: int(row.Level), XPTotal: int(row.XPTotal),
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

// FirstID returns the id of the first row in characters; used at startup when
// the app runs in single-character mode. Returns ErrCharacterNotFound when empty.
func (d *DAL) FirstID(ctx context.Context) (int64, error) {
	id, err := queries.GetFirstCharacterID().One(ctx, d.exec())
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("first character id: %w", err)
	}
	return id, nil
}

func decodeLanguages(raw string) ([]string, error) {
	langs := []string{}
	if err := json.Unmarshal([]byte(raw), &langs); err != nil {
		return nil, fmt.Errorf("decode languages: %w", err)
	}
	return langs, nil
}

func (d *DAL) execCtx(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// GetDetails reads the editable details, joining the transaction carried by
// ctx. Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetDetails(ctx context.Context, id int64) (charactersvc.Details, error) {
	r, err := queries.GetCharacterDetails(id).One(ctx, d.execCtx(ctx))
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
	ids, err := queries.UpdateCharacterDetails(v.Race, v.Alignment, int64(v.Age), v.Sex, v.Height,
		v.Weight, v.Hair, v.Eyes, string(langs), id).All(ctx, d.execCtx(ctx))
	if err != nil {
		return fmt.Errorf("update character details: %w", err)
	}
	if len(ids) == 0 {
		return domain.ErrCharacterNotFound
	}
	return nil
}

// Package characterdal is the storage adapter for charactersvc. It delegates
// to bob-generated typed queries in the queries/ subpackage.
package characterdal

import (
	"context"
	"database/sql"
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

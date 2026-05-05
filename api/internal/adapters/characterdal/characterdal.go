// Package characterdal is the storage adapter for charactersvc.
package characterdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

// GetCharacter reads the row by id and maps it to a domain Character.
// Returns domain.ErrCharacterNotFound when the row is missing.
func (d *DAL) GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error) {
	var c charactersvc.Character
	err := d.q.QueryRowContext(ctx,
		`SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
		        save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell
		   FROM characters WHERE id = ?`, id,
	).Scan(
		&c.ID, &c.Name, &c.Str, &c.Dex, &c.Con, &c.Int, &c.Wis, &c.Cha,
		&c.HPMax, &c.Thac0Base, &c.Level, &c.XPTotal,
		&c.SaveParaPoison, &c.SaveRodStaffWand, &c.SavePetriPoly, &c.SaveBreath, &c.SaveSpell,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	return &c, nil
}

// FirstID returns the id of the first row in characters; used at startup when
// the app runs in single-character mode. Returns ErrCharacterNotFound when
// empty.
func (d *DAL) FirstID(ctx context.Context) (int64, error) {
	var id int64
	err := d.q.QueryRowContext(ctx, "SELECT id FROM characters ORDER BY id LIMIT 1").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrCharacterNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("first character id: %w", err)
	}
	return id, nil
}

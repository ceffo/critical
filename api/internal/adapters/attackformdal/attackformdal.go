// Package attackformdal is the storage adapter for the attack form's last
// choices (combatsvc.FormStore). It delegates to bob-generated typed queries.
package attackformdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/attackformdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of combatsvc.FormStore.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// Get returns the character's stored form; ok is false when none was saved.
func (d *DAL) Get(ctx context.Context, characterID int64) (combatsvc.AttackForm, bool, error) {
	row, err := queries.GetAttackForm(characterID).One(ctx, d.exec(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return combatsvc.AttackForm{}, false, nil
	}
	if err != nil {
		return combatsvc.AttackForm{}, false, fmt.Errorf("query attack_forms: %w", err)
	}
	return combatsvc.AttackForm{
		WeaponID:         row.WeaponID,
		PreviousWeaponID: row.PreviousWeaponID,
		AmmoItemID:       row.AmmoItemID,
		TargetAC:         int(row.TargetAc),
		TargetSize:       combatsvc.TargetSize(row.TargetSize),
		Situational:      combatsvc.Situational{Value: int(row.SituationalValue), Label: row.SituationalLabel},
	}, true, nil
}

// Put upserts the character's form.
func (d *DAL) Put(ctx context.Context, characterID int64, f combatsvc.AttackForm) error {
	_, err := queries.UpsertAttackForm([]queries.UpsertAttackForm_AttackForms{{
		CharacterID:      characterID,
		WeaponID:         f.WeaponID,
		PreviousWeaponID: f.PreviousWeaponID,
		AmmoItemID:       f.AmmoItemID,
		TargetAc:         int64(f.TargetAC),
		TargetSize:       string(f.TargetSize),
		SituationalValue: int64(f.Situational.Value),
		SituationalLabel: f.Situational.Label,
	}}).Exec(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("upsert attack_forms: %w", err)
	}
	return nil
}

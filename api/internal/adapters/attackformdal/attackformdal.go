// Package attackformdal is the storage adapter for the attack form's last
// choices (combatsvc.FormStore). It delegates to sqlc-generated queries.
package attackformdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/attackformdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// DAL is the implementation of combatsvc.FormStore over the sqlc-generated
// queries.
type DAL struct {
	q queries.Querier
}

var _ combatsvc.FormStore = (*DAL)(nil)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// Get returns the character's stored form; ok is false when none was saved.
func (d *DAL) Get(ctx context.Context, characterID int64) (combatsvc.AttackForm, bool, error) {
	row, err := d.q.GetAttackForm(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return combatsvc.AttackForm{}, false, nil
	}
	if err != nil {
		return combatsvc.AttackForm{}, false, fmt.Errorf("query attack_forms: %w", err)
	}
	f := combatsvc.AttackForm{
		WeaponID:         row.WeaponID,
		PreviousWeaponID: row.PreviousWeaponID,
		AmmoItemID:       row.AmmoItemID,
		TargetSize:       combatsvc.TargetSize(row.TargetSize),
		Situational:      combatsvc.Situational{Value: int(row.SituationalValue), Label: row.SituationalLabel},
	}
	if row.TargetAcKnown != 0 {
		ac := int(row.TargetAc)
		f.TargetAC = &ac
	}
	return f, true, nil
}

// Put upserts the character's form. An unknown target AC is stored as
// target_ac_known = 0 (target_ac 0, ignored).
func (d *DAL) Put(ctx context.Context, characterID int64, f combatsvc.AttackForm) error {
	var ac, known int64
	if f.TargetAC != nil {
		ac, known = int64(*f.TargetAC), 1
	}
	err := d.q.UpsertAttackForm(ctx, queries.UpsertAttackFormParams{
		CharacterID:      characterID,
		WeaponID:         f.WeaponID,
		PreviousWeaponID: f.PreviousWeaponID,
		AmmoItemID:       f.AmmoItemID,
		TargetAc:         ac,
		TargetAcKnown:    known,
		TargetSize:       string(f.TargetSize),
		SituationalValue: int64(f.Situational.Value),
		SituationalLabel: f.Situational.Label,
	})
	if err != nil {
		return fmt.Errorf("upsert attack_forms: %w", err)
	}
	return nil
}

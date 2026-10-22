// Package rulesetdal is the storage adapter of the ruleset (rulesetsvc.DAL):
// the default ruleset, its crit rows, success effects and the audit of its
// edits. It delegates to sqlc-generated queries.
package rulesetdal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/rulesetdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/rulesetsvc"
)

var _ rulesetsvc.DAL = (*DAL)(nil)

// DAL is the implementation of rulesetsvc.DAL over the sqlc-generated
// queries.
type DAL struct {
	q queries.Querier
}

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// Default returns the default ruleset with its crit rows and success effects.
func (d *DAL) Default(ctx context.Context) (ruleset.Ruleset, error) {
	row, err := d.q.RulesetByName(ctx, ruleset.DefaultName)
	if errors.Is(err, sql.ErrNoRows) {
		return ruleset.Ruleset{}, fmt.Errorf("the default ruleset: %w", domain.ErrNotFound)
	}
	if err != nil {
		return ruleset.Ruleset{}, fmt.Errorf("read the default ruleset: %w", err)
	}
	crit, err := d.q.ListRulesetCritRows(ctx, row.ID)
	if err != nil {
		return ruleset.Ruleset{}, fmt.Errorf("list the crit rows: %w", err)
	}
	effects, err := d.q.ListRulesetSuccessEffects(ctx, row.ID)
	if err != nil {
		return ruleset.Ruleset{}, fmt.Errorf("list the success effects: %w", err)
	}
	out := ruleset.Ruleset{
		ID: row.ID, Name: row.Name, CatalogueVersion: row.CatalogueVersion,
		Crit:        ruleset.CritTable{Placeholder: row.CritPlaceholder, Rows: make([]ruleset.CritRow, 0, len(crit))},
		RacialLift:  ruleset.RacialLift{MulticlassAnyRace: row.MulticlassAnyRace, DualclassAnyRace: row.DualclassAnyRace},
		WeaponLift:  ruleset.WeaponLift{MulticlassAnyWeapon: row.MulticlassAnyWeapon},
		LevelLimits: ruleset.LevelLimitsMode(row.LevelLimits),
	}
	for _, r := range crit {
		out.Crit.Rows = append(out.Crit.Rows, ruleset.CritRow{From: int(r.RollFrom), To: int(r.RollTo), Multiplier: r.Multiplier})
	}
	out.SuccessEffects = make([]ruleset.SuccessEffect, 0, len(effects))
	for _, e := range effects {
		out.SuccessEffects = append(out.SuccessEffects, ruleset.SuccessEffect{ProficiencyID: e.ProficiencyID, AC: int(e.Ac), Rounds: int(e.Rounds)})
	}
	return out, nil
}

// ReplaceCritTable swaps the whole table: it clears it, then inserts the rows
// in order, and clears the placeholder flag. The caller runs it in a
// transaction so a failed insert leaves the previous table in place.
func (d *DAL) ReplaceCritTable(ctx context.Context, rulesetID int64, rows []ruleset.CritRow) error {
	if err := d.q.DeleteRulesetCritRows(ctx, rulesetID); err != nil {
		return fmt.Errorf("clear the crit table: %w", err)
	}
	for _, r := range rows {
		err := d.q.InsertRulesetCritRow(ctx, queries.InsertRulesetCritRowParams{
			RulesetID: rulesetID, RollFrom: int64(r.From), RollTo: int64(r.To), Multiplier: r.Multiplier,
		})
		if err != nil {
			return fmt.Errorf("insert crit row %d-%d: %w", r.From, r.To, err)
		}
	}
	if err := d.q.SetRulesetCritPlaceholder(ctx, queries.SetRulesetCritPlaceholderParams{ID: rulesetID, CritPlaceholder: false}); err != nil {
		return fmt.Errorf("clear the crit placeholder: %w", err)
	}
	return nil
}

// SetRacialLift stores the two switches.
func (d *DAL) SetRacialLift(ctx context.Context, rulesetID int64, lift ruleset.RacialLift) error {
	err := d.q.SetRulesetRacialLift(ctx, queries.SetRulesetRacialLiftParams{
		ID: rulesetID, MulticlassAnyRace: lift.MulticlassAnyRace, DualclassAnyRace: lift.DualclassAnyRace,
	})
	if err != nil {
		return fmt.Errorf("set the racial lift: %w", err)
	}
	return nil
}

// SetWeaponLift stores the weapon lift switch.
func (d *DAL) SetWeaponLift(ctx context.Context, rulesetID int64, lift ruleset.WeaponLift) error {
	err := d.q.SetRulesetWeaponLift(ctx, queries.SetRulesetWeaponLiftParams{
		ID: rulesetID, MulticlassAnyWeapon: lift.MulticlassAnyWeapon,
	})
	if err != nil {
		return fmt.Errorf("set the weapon lift: %w", err)
	}
	return nil
}

// ReplaceSuccessEffects swaps the whole list of effects.
func (d *DAL) ReplaceSuccessEffects(ctx context.Context, rulesetID int64, effects []ruleset.SuccessEffect) error {
	if err := d.q.DeleteRulesetSuccessEffects(ctx, rulesetID); err != nil {
		return fmt.Errorf("clear the success effects: %w", err)
	}
	for _, e := range effects {
		err := d.q.InsertRulesetSuccessEffect(ctx, queries.InsertRulesetSuccessEffectParams{
			RulesetID: rulesetID, ProficiencyID: e.ProficiencyID, Ac: int64(e.AC), Rounds: int64(e.Rounds),
		})
		if err != nil {
			return fmt.Errorf("insert the success effect of %s: %w", e.ProficiencyID, err)
		}
	}
	return nil
}

// SetCatalogueVersion records the catalogue version the ruleset reads.
func (d *DAL) SetCatalogueVersion(ctx context.Context, rulesetID int64, version string) error {
	err := d.q.SetRulesetCatalogueVersion(ctx, queries.SetRulesetCatalogueVersionParams{ID: rulesetID, CatalogueVersion: version})
	if err != nil {
		return fmt.Errorf("set the catalogue version: %w", err)
	}
	return nil
}

// AddChange appends one audit record.
func (d *DAL) AddChange(ctx context.Context, c ruleset.Change) error {
	err := d.q.InsertRulesetChange(ctx, queries.InsertRulesetChangeParams{
		RulesetID: c.RulesetID, ChangedBy: c.By, Rule: c.Rule, BeforeValue: c.Before, AfterValue: c.After,
	})
	if err != nil {
		return fmt.Errorf("insert the ruleset change: %w", err)
	}
	return nil
}

// ListChanges lists the ruleset's audit, newest first.
func (d *DAL) ListChanges(ctx context.Context, rulesetID int64) ([]ruleset.Change, error) {
	rows, err := d.q.ListRulesetChanges(ctx, rulesetID)
	if err != nil {
		return nil, fmt.Errorf("list the ruleset changes: %w", err)
	}
	out := make([]ruleset.Change, 0, len(rows))
	for _, r := range rows {
		out = append(out, ruleset.Change{
			ID: r.ID, RulesetID: r.RulesetID, At: r.ChangedAt, By: r.ChangedBy, Rule: r.Rule,
			Before: r.BeforeValue, After: r.AfterValue,
		})
	}
	return out, nil
}

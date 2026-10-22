// Package rulesetsvc serves and edits the ruleset (ADR 0041): the default
// ruleset every character plays under, its house rules, and the audit of
// their edits. It is also the Resolver the rules services take their ruleset
// from (step 3).
package rulesetsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	ruleset.Resolver
	// Get returns the default ruleset, house rules included.
	Get(ctx context.Context) (ruleset.Ruleset, error)
	// Changes lists the audit of the default ruleset's edits, newest first.
	Changes(ctx context.Context) ([]ruleset.Change, error)
	// RecordCatalogueVersion records the catalogue version the server loaded
	// on the default ruleset (step 1). It is startup bookkeeping, not an
	// edit: no change record.
	RecordCatalogueVersion(ctx context.Context, version string) error
	// SetCritTable validates and stores the whole crit table (step 2.1). An
	// invalid table is refused and the stored one stays.
	SetCritTable(ctx context.Context, by uuid.UUID, rows []ruleset.CritRow) (ruleset.CritTable, error)
	// SetRacialLift stores the two racial lift switches (step 2.2).
	SetRacialLift(ctx context.Context, by uuid.UUID, lift ruleset.RacialLift) (ruleset.RacialLift, error)
	// SetSuccessEffects replaces the success effects (step 2.4); a proficiency
	// the book does not know, or an effect of zero rounds, is refused.
	SetSuccessEffects(ctx context.Context, by uuid.UUID, effects []ruleset.SuccessEffect) ([]ruleset.SuccessEffect, error)
}

// DAL is the storage port. Every method joins the ctx transaction.
type DAL interface {
	// Default returns the default ruleset with its child rows.
	Default(ctx context.Context) (ruleset.Ruleset, error)
	// ReplaceCritTable swaps the crit rows; the table is no longer the
	// placeholder.
	ReplaceCritTable(ctx context.Context, rulesetID int64, rows []ruleset.CritRow) error
	SetRacialLift(ctx context.Context, rulesetID int64, lift ruleset.RacialLift) error
	ReplaceSuccessEffects(ctx context.Context, rulesetID int64, effects []ruleset.SuccessEffect) error
	SetCatalogueVersion(ctx context.Context, rulesetID int64, version string) error
	// AddChange appends one audit record; the store stamps its id and time.
	AddChange(ctx context.Context, c ruleset.Change) error
	// ListChanges lists the ruleset's audit, newest first.
	ListChanges(ctx context.Context, rulesetID int64) ([]ruleset.Change, error)
}

// Proficiencies tells which proficiency ids the book knows.
type Proficiencies interface {
	Has(id string) bool
}

// Deps are the service's collaborators.
type Deps struct {
	DAL           DAL
	Proficiencies Proficiencies
	Tx            domain.TxRunner
}

type service struct {
	dal   DAL
	profs Proficiencies
	tx    domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Proficiencies == nil || d.Tx == nil {
		return nil, fmt.Errorf("rulesetsvc: nil dependency")
	}
	return &service{dal: d.DAL, profs: d.Proficiencies, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares nothing: an edit is no character's action, it has no undo
// and no journal entry (ADR 0041 step 7).
func (s *service) Module() domain.Module { return domain.Module{Name: "rulesetsvc"} }

func (s *service) Get(ctx context.Context) (ruleset.Ruleset, error) {
	rs, err := s.dal.Default(ctx)
	if err != nil {
		return ruleset.Ruleset{}, fmt.Errorf("load the default ruleset: %w", err)
	}
	return rs, nil
}

// ForCharacter is the one place that says every character plays under the
// default ruleset (ADR 0041 steps 3 and 9). When games arrive it reads the
// character's game's ruleset instead.
func (s *service) ForCharacter(ctx context.Context, _ int64) (ruleset.Ruleset, error) {
	return s.Get(ctx)
}

func (s *service) Changes(ctx context.Context) ([]ruleset.Change, error) {
	rs, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	out, err := s.dal.ListChanges(ctx, rs.ID)
	if err != nil {
		return nil, fmt.Errorf("list the ruleset changes: %w", err)
	}
	return out, nil
}

func (s *service) RecordCatalogueVersion(ctx context.Context, version string) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		rs, err := s.Get(ctx)
		if err != nil {
			return err
		}
		if rs.CatalogueVersion == version {
			return nil
		}
		if err := s.dal.SetCatalogueVersion(ctx, rs.ID, version); err != nil {
			return fmt.Errorf("record the catalogue version: %w", err)
		}
		return nil
	})
}

// edit runs one house rule's edit in a transaction: it loads the ruleset,
// lets change write the new value and return the rule's value before and
// after, and appends the audit record, all or nothing (ADR 0041 step 7).
func (s *service) edit(ctx context.Context, by uuid.UUID, rule string,
	change func(ctx context.Context, rs ruleset.Ruleset) (before, after any, err error),
) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		rs, err := s.Get(ctx)
		if err != nil {
			return err
		}
		before, after, err := change(ctx, rs)
		if err != nil {
			return err
		}
		b, err := json.Marshal(before)
		if err != nil {
			return fmt.Errorf("encode the value before: %w", err)
		}
		a, err := json.Marshal(after)
		if err != nil {
			return fmt.Errorf("encode the value after: %w", err)
		}
		err = s.dal.AddChange(ctx, ruleset.Change{RulesetID: rs.ID, By: by, Rule: rule, Before: string(b), After: string(a)})
		if err != nil {
			return fmt.Errorf("record the %s change: %w", rule, err)
		}
		return nil
	})
}

func (s *service) SetCritTable(ctx context.Context, by uuid.UUID, rows []ruleset.CritRow) (ruleset.CritTable, error) {
	sorted, err := ruleset.ValidateCritRows(rows)
	if err != nil {
		return ruleset.CritTable{}, err
	}
	err = s.edit(ctx, by, ruleset.RuleCritTable, func(ctx context.Context, rs ruleset.Ruleset) (any, any, error) {
		if err := s.dal.ReplaceCritTable(ctx, rs.ID, sorted); err != nil {
			return nil, nil, fmt.Errorf("save the crit table: %w", err)
		}
		return critDoc(rs.Crit.Rows), critDoc(sorted), nil
	})
	if err != nil {
		return ruleset.CritTable{}, err
	}
	return ruleset.CritTable{Rows: sorted}, nil
}

func (s *service) SetRacialLift(ctx context.Context, by uuid.UUID, lift ruleset.RacialLift) (ruleset.RacialLift, error) {
	err := s.edit(ctx, by, ruleset.RuleRacialLift, func(ctx context.Context, rs ruleset.Ruleset) (any, any, error) {
		if err := s.dal.SetRacialLift(ctx, rs.ID, lift); err != nil {
			return nil, nil, fmt.Errorf("save the racial lift: %w", err)
		}
		return liftDoc(rs.RacialLift), liftDoc(lift), nil
	})
	if err != nil {
		return ruleset.RacialLift{}, err
	}
	return lift, nil
}

func (s *service) SetSuccessEffects(ctx context.Context, by uuid.UUID, effects []ruleset.SuccessEffect) ([]ruleset.SuccessEffect, error) {
	sorted, err := ruleset.ValidateSuccessEffects(effects, s.profs.Has)
	if err != nil {
		return nil, err
	}
	err = s.edit(ctx, by, ruleset.RuleSuccessEffects, func(ctx context.Context, rs ruleset.Ruleset) (any, any, error) {
		if err := s.dal.ReplaceSuccessEffects(ctx, rs.ID, sorted); err != nil {
			return nil, nil, fmt.Errorf("save the success effects: %w", err)
		}
		return effectsDoc(rs.SuccessEffects), effectsDoc(sorted), nil
	})
	if err != nil {
		return nil, err
	}
	return sorted, nil
}

// The audit's before and after are JSON documents; these are their shapes,
// kept here so the domain types carry no wire tags.
type critRowDoc struct {
	From       int     `json:"from"`
	To         int     `json:"to"`
	Multiplier float64 `json:"multiplier"`
}

func critDoc(rows []ruleset.CritRow) []critRowDoc {
	out := make([]critRowDoc, 0, len(rows))
	for _, r := range rows {
		out = append(out, critRowDoc{From: r.From, To: r.To, Multiplier: r.Multiplier})
	}
	return out
}

type liftDocument struct {
	MulticlassAnyRace bool `json:"multiclassAnyRace"`
	DualclassAnyRace  bool `json:"dualclassAnyRace"`
}

func liftDoc(l ruleset.RacialLift) liftDocument {
	return liftDocument{MulticlassAnyRace: l.MulticlassAnyRace, DualclassAnyRace: l.DualclassAnyRace}
}

type effectDoc struct {
	ProficiencyID string `json:"proficiencyId"`
	AC            int    `json:"ac"`
	Rounds        int    `json:"rounds"`
}

func effectsDoc(effects []ruleset.SuccessEffect) []effectDoc {
	out := make([]effectDoc, 0, len(effects))
	for _, e := range effects {
		out = append(out, effectDoc{ProficiencyID: e.ProficiencyID, AC: e.AC, Rounds: e.Rounds})
	}
	return out
}

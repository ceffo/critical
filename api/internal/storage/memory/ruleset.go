package memory

import (
	"context"
	"slices"
	"time"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/rulesetsvc"
)

// Rulesets is the ruleset store (rulesetsvc.DAL). A new store holds the
// default ruleset, as a migrated database does.
type Rulesets struct{ s *Store }

var _ rulesetsvc.DAL = Rulesets{}

// Rulesets returns the ruleset port.
func (s *Store) Rulesets() Rulesets { return Rulesets{s} }

// Default returns the default ruleset.
func (p Rulesets) Default(ctx context.Context) (ruleset.Ruleset, error) {
	var out ruleset.Ruleset
	err := p.s.read(ctx, func(t *tables) error {
		out = clone(t.ruleset)
		return nil
	})
	return out, err
}

func (p Rulesets) own(t *tables, id int64) error {
	if id != t.ruleset.ID {
		return domain.ErrNotFound
	}
	return nil
}

// ReplaceCritTable swaps the whole table; two rows starting at the same roll
// are refused (the range start is the table's key).
func (p Rulesets) ReplaceCritTable(ctx context.Context, rulesetID int64, rows []ruleset.CritRow) error {
	return p.s.write(ctx, func(t *tables) error {
		if err := p.own(t, rulesetID); err != nil {
			return err
		}
		next := slices.Clone(rows)
		slices.SortStableFunc(next, func(a, b ruleset.CritRow) int { return a.From - b.From })
		for i := 1; i < len(next); i++ {
			if next[i].From == next[i-1].From {
				return errDuplicate("ruleset_crit_rows", next[i].From)
			}
		}
		t.ruleset.Crit = ruleset.CritTable{Rows: next}
		return nil
	})
}

// SetRacialLift stores the two racial lift switches.
func (p Rulesets) SetRacialLift(ctx context.Context, rulesetID int64, lift ruleset.RacialLift) error {
	return p.s.write(ctx, func(t *tables) error {
		if err := p.own(t, rulesetID); err != nil {
			return err
		}
		t.ruleset.RacialLift = lift
		return nil
	})
}

// ReplaceSuccessEffects swaps the whole list of effects, sorted by proficiency; a proficiency twice is refused.
func (p Rulesets) ReplaceSuccessEffects(ctx context.Context, rulesetID int64, effects []ruleset.SuccessEffect) error {
	return p.s.write(ctx, func(t *tables) error {
		if err := p.own(t, rulesetID); err != nil {
			return err
		}
		next := slices.Clone(effects)
		slices.SortFunc(next, func(a, b ruleset.SuccessEffect) int {
			switch {
			case a.ProficiencyID < b.ProficiencyID:
				return -1
			case a.ProficiencyID > b.ProficiencyID:
				return 1
			}
			return 0
		})
		for i := 1; i < len(next); i++ {
			if next[i].ProficiencyID == next[i-1].ProficiencyID {
				return errDuplicate("ruleset_success_effects", next[i].ProficiencyID)
			}
		}
		t.ruleset.SuccessEffects = next
		return nil
	})
}

// SetCatalogueVersion records the catalogue version the ruleset reads.
func (p Rulesets) SetCatalogueVersion(ctx context.Context, rulesetID int64, version string) error {
	return p.s.write(ctx, func(t *tables) error {
		if err := p.own(t, rulesetID); err != nil {
			return err
		}
		t.ruleset.CatalogueVersion = version
		return nil
	})
}

// AddChange appends one audit record, stamping its id and time.
func (p Rulesets) AddChange(ctx context.Context, c ruleset.Change) error {
	return p.s.write(ctx, func(t *tables) error {
		if err := p.own(t, c.RulesetID); err != nil {
			return err
		}
		c.ID = int64(len(t.rulesetChanges) + 1)
		c.At = time.Now().UTC()
		t.rulesetChanges = append(t.rulesetChanges, c)
		return nil
	})
}

// ListChanges lists the ruleset's audit, newest first.
func (p Rulesets) ListChanges(ctx context.Context, rulesetID int64) ([]ruleset.Change, error) {
	out := []ruleset.Change{}
	err := p.s.read(ctx, func(t *tables) error {
		if err := p.own(t, rulesetID); err != nil {
			return err
		}
		for _, c := range slices.Backward(t.rulesetChanges) {
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

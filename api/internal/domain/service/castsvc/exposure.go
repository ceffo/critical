package castsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// ExposureAt reads the exposure damage of spellID at casterLevel from the
// effect entry (ADR 0008 step 16): the dice Expose rolls, left unrolled.
func (s *service) ExposureAt(_ context.Context, spellID string, casterLevel int) (*spelleffectsvc.ExposureAt, error) {
	if err := checkCasterLevel(casterLevel); err != nil {
		return nil, err
	}
	e, ok := s.Effects.Get(spellID)
	if !ok {
		return nil, nil
	}
	return e.ExposureAt(casterLevel)
}

// Expose rolls one round of exposure to a spell's area at the caster level
// (ADR 0008 step 16): exactly the dice served for that round, once. It is one
// exposure entry; when the Bard is the one exposed the entry's HP delta takes
// the total off his HP (the HP ledger, ADR 0007), so popping it gives the HP
// back. Nothing is rolled or logged for a refused request.
func (s *service) Expose(ctx context.Context, characterID int64, req ExposureRequest) (ExposureResult, error) {
	id := strings.TrimSpace(req.SpellID)
	if id == "" {
		return ExposureResult{}, fmt.Errorf("%w: spell id is required", domain.ErrInvalidInput)
	}
	if err := checkCasterLevel(req.CasterLevel); err != nil {
		return ExposureResult{}, err
	}
	round := req.Round
	if round == 0 {
		round = 1
	}
	if round < 1 || round > spelleffectsvc.MaxExposureRound {
		return ExposureResult{}, fmt.Errorf("%w: the round of exposure must be between 1 and %d", domain.ErrInvalidInput, spelleffectsvc.MaxExposureRound)
	}
	entry, ok := s.Effects.Get(id)
	if !ok {
		return ExposureResult{}, fmt.Errorf("%w: %s", spelleffectsvc.ErrNoExposure, id)
	}
	damage, mode, err := entry.ExposureFor(req.Mode)
	if err != nil {
		return ExposureResult{}, err
	}
	res := ExposureResult{
		SpellID: entry.ID, Name: entry.Name, CasterLevel: req.CasterLevel,
		Mode: mode, Round: round, OnSelf: req.OnSelf,
	}
	if res.Exposure, err = entry.ExposureAt(req.CasterLevel); err != nil {
		return ExposureResult{}, fmt.Errorf("%s: %w", entry.ID, err)
	}
	if res.Damage, err = s.rollExposure(entry, damage, req.CasterLevel, round); err != nil {
		return ExposureResult{}, fmt.Errorf("%s exposure: %w", entry.ID, err)
	}
	payload, err := json.Marshal(toExposureRecord(res))
	if err != nil {
		return ExposureResult{}, fmt.Errorf("encode exposure payload: %w", err)
	}
	e := actionlog.NewEntry{CharacterID: characterID, Kind: actionlog.KindExposure, Payload: payload}
	if !req.OnSelf {
		if res.EntryID, err = s.Log.Record(ctx, e); err != nil {
			return ExposureResult{}, fmt.Errorf("record exposure: %w", err)
		}
		return res, nil
	}
	// On the Bard: the ledger applies the damage (down to the floor of ADR
	// 0011); the HP total before and after, in the recording transaction,
	// is what it applied.
	e.HPDelta = -res.Damage.Total
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.Log.HPTotal(ctx, characterID)
		if err != nil {
			return fmt.Errorf("hp total: %w", err)
		}
		if res.EntryID, err = s.Log.Record(ctx, e); err != nil {
			return fmt.Errorf("record exposure: %w", err)
		}
		after, err := s.Log.HPTotal(ctx, characterID)
		if err != nil {
			return fmt.Errorf("hp total: %w", err)
		}
		res.HPDelta = after - before
		return nil
	})
	if err != nil {
		return ExposureResult{}, err
	}
	return res, nil
}

// rollExposure rolls one round of exposure: for a damage made of steps, the
// step of that round (the last one for a later round), else the damage's
// dice once (parts each once, step 13).
func (s *service) rollExposure(e spelleffectsvc.Entry, d *spelleffectsvc.Damage, level, round int) (*DamageResult, error) {
	step, stepped, err := d.ExposureStep(level, round)
	if err != nil {
		return nil, err
	}
	if !stepped {
		return s.rollDamage(e, d, level)
	}
	f := d.Steps.FormulaOf(step)
	r := Roll{Formula: f.String(), Step: step}
	r.Total, err = f.EvalStep(level, step, func(sides int) int {
		v := s.Dice.RollDie(sides)
		r.Dice = append(r.Dice, DieRoll{Sides: sides, Value: v})
		return v
	})
	if err != nil {
		return nil, fmt.Errorf("roll %s at level %d step %d: %w", f, level, step, err)
	}
	return &DamageResult{
		Type: d.Type, Instances: 1, Rounds: 1, StepsPer: spelleffectsvc.StepPerRound,
		Rolls: []Roll{r}, Total: r.Total,
	}, nil
}

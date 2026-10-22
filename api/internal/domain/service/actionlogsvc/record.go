package actionlogsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Record validates and stores an entry with its snapshots in one transaction,
// joining the caller's transaction when ctx carries one. Round 0 is stamped
// with the current round. A positive HPDelta (a heal, from any service) is
// capped at max HP before it is stored (actionlog.CapHeal, ADR 0007 steps 4 and 5) and the
// payload gets the requested vs applied breakdown. A negative HPDelta (damage)
// is clamped so HP stops at the floor (actionlog.FloorDamage, ADR 0011 step 1);
// a clamped entry gets the breakdown too. A PSPDelta is bounded the same way
// (ADR 0043 step 5, actionlog.CapPSP): a gain capped at the PSPs spent, a loss
// floored so the pool never goes under 0; a changed entry carries
// pspRequested and pspApplied.
//
// The entry joins the action-log transaction of the request (ADR 0018 step
// 1): the first entry recorded inside an outermost InTx of a runner wrapped by
// Transactions starts it, every later one in the same InTx shares its id.
func (s *service) Record(ctx context.Context, e actionlog.NewEntry) (int64, error) {
	if e.Kind == "" {
		return 0, fmt.Errorf("%w: kind is required", domain.ErrInvalidInput)
	}
	e.Summary = strings.TrimSpace(e.Summary)
	if e.Summary == "" && len(e.Payload) == 0 {
		return 0, fmt.Errorf("%w: an entry needs a payload or a summary", domain.ErrInvalidInput)
	}
	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage(`{}`)
	}
	seen := map[string]bool{}
	for _, snap := range e.Snapshots {
		if snap.Key == "" || seen[snap.Key] {
			return 0, fmt.Errorf("%w: snapshot keys must be non-empty and unique", domain.ErrInvalidInput)
		}
		seen[snap.Key] = true
	}

	var id int64
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if e.Round == 0 {
			round, err := s.rounds.GetRound(ctx, e.CharacterID)
			if err != nil {
				return fmt.Errorf("current round: %w", err)
			}
			e.Round = round
		}
		switch {
		case e.HPDelta > 0:
			if err := s.capHeal(ctx, &e); err != nil {
				return err
			}
		case e.HPDelta < 0:
			if err := s.floorDamage(ctx, &e); err != nil {
				return err
			}
		}
		if e.PSPDelta != 0 {
			if err := s.boundPSP(ctx, &e); err != nil {
				return err
			}
		}
		var err error
		id, err = s.insert(ctx, e)
		if err != nil {
			return fmt.Errorf("insert entry: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// capHeal clips e's heal to the gap to max HP and records the breakdown in its
// payload (ADR 0007 steps 4 and 5). It must run inside the recording transaction so the
// total it reads is the one the entry is added to.
func (s *service) capHeal(ctx context.Context, e *actionlog.NewEntry) error {
	total, err := s.dal.HPTotal(ctx, e.CharacterID)
	if err != nil {
		return fmt.Errorf("hp total: %w", err)
	}
	b := actionlog.HPBreakdown{Requested: e.HPDelta, Applied: actionlog.CapHeal(total, e.HPDelta)}
	payload, err := actionlog.WithHPBreakdown(e.Payload, b)
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrInvalidInput, err)
	}
	e.HPDelta, e.Payload = b.Applied, payload
	return nil
}

// floorDamage clips e's damage so current HP stops at actionlog.HPFloor and,
// when it clipped, records the breakdown in its payload (ADR 0011 step 1). Like
// capHeal it runs inside the recording transaction, so the total it reads is
// the one the entry is added to and the ledger SUM stays the truth.
func (s *service) floorDamage(ctx context.Context, e *actionlog.NewEntry) error {
	maxHP, err := s.maxima.GetMaxHP(ctx, e.CharacterID)
	if err != nil {
		return fmt.Errorf("max hp: %w", err)
	}
	total, err := s.dal.HPTotal(ctx, e.CharacterID)
	if err != nil {
		return fmt.Errorf("hp total: %w", err)
	}
	applied := actionlog.FloorDamage(maxHP, total, e.HPDelta)
	if applied == e.HPDelta {
		return nil
	}
	payload, err := actionlog.WithHPBreakdown(e.Payload, actionlog.HPBreakdown{Requested: e.HPDelta, Applied: applied})
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrInvalidInput, err)
	}
	e.HPDelta, e.Payload = applied, payload
	return nil
}

// boundPSP clips e's PSP delta to the pool's bounds (actionlog.CapPSP, ADR
// 0043 step 5) and, when it clipped, records the breakdown in its payload.
// Like capHeal it runs inside the recording transaction, so the total it reads
// is the one the entry is added to.
func (s *service) boundPSP(ctx context.Context, e *actionlog.NewEntry) error {
	maxPSP, err := s.maxima.GetMaxPSP(ctx, e.CharacterID)
	if err != nil {
		return fmt.Errorf("max psp: %w", err)
	}
	total, err := s.dal.PSPTotal(ctx, e.CharacterID)
	if err != nil {
		return fmt.Errorf("psp total: %w", err)
	}
	applied := actionlog.CapPSP(maxPSP, total, e.PSPDelta)
	if applied == e.PSPDelta {
		return nil
	}
	payload, err := actionlog.WithPSPBreakdown(e.Payload, actionlog.PSPBreakdown{Requested: e.PSPDelta, Applied: applied})
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrInvalidInput, err)
	}
	e.PSPDelta, e.Payload = applied, payload
	return nil
}

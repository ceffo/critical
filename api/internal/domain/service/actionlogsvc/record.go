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
// a clamped entry gets the breakdown too.
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
		var err error
		id, err = s.dal.InsertEntry(ctx, e)
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
	maxHP, err := s.maxHP.GetMaxHP(ctx, e.CharacterID)
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

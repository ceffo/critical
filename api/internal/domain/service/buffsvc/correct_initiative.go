package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// InitiativeCorrector replaces the round's initiative by a spell cast's or
// an item used's term (ADR 0035 item 6.2). It needs the initiative store, the
// round counter and the log, and nothing of the buffs or the items: the items
// service is given one, built before both services, so neither is wired
// with a dependency bound later.
type InitiativeCorrector interface {
	CorrectInitiative(ctx context.Context, characterID int64, cause InitiativeModifier) (*Initiative, error)
}

// CorrectionStore is the slice of DAL the correction reads and writes.
type CorrectionStore interface {
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
	GetInitiative(ctx context.Context, characterID int64) (*Initiative, error)
	SaveInitiative(ctx context.Context, characterID int64, initiative *Initiative) error
}

type corrector struct {
	dal CorrectionStore
	log actionlog.Recorder
}

// NewInitiativeCorrector constructs the corrector over the store and the log.
func NewInitiativeCorrector(dal CorrectionStore, log actionlog.Recorder) (InitiativeCorrector, error) {
	switch {
	case dal == nil:
		return nil, fmt.Errorf("buffsvc: nil initiative store")
	case log == nil:
		return nil, fmt.Errorf("buffsvc: nil log")
	}
	return corrector{dal: dal, log: log}, nil
}

// CorrectInitiative is ADR 0035 item 6.2: a spell cast or a magic item used
// in a round replaces that round's initiative by the same d10 plus the
// cause's term (SpellInitiative, ItemInitiative) and the effects recorded at
// the roll; the weapon term, or an earlier correction's cause, goes. No die
// is rolled. It records an initiative_correct entry stamped with the round,
// whose snapshot keeps the replaced initiative as "before" (undo gives it
// back), and stores the new one with that entry's id. It runs inside the
// caller's transaction (ctx) and must be the last entry the caller records,
// so a reroll stays allowed right after it. Out of combat or with no
// initiative it does nothing and returns nil.
func (c corrector) CorrectInitiative(ctx context.Context, characterID int64, cause InitiativeModifier) (*Initiative, error) {
	if cause.Source != InitiativeSpell && cause.Source != InitiativeItem {
		return nil, fmt.Errorf("%w: an initiative correction comes from a spell or an item, not %q", domain.ErrInvalidInput, cause.Source)
	}
	round, err := c.dal.GetRoundNumber(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get round: %w", err)
	}
	current, err := c.dal.GetInitiative(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get initiative: %w", err)
	}
	if current == nil || round == 0 {
		return nil, nil
	}
	next := Initiative{D10: current.D10, Modifiers: append([]InitiativeModifier{cause}, effectModifiers(current.Modifiers)...)}
	snap, err := initiativeSnapshot(current, &next)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(initiativeCorrectPayload{
		Round: round, D10: next.D10, Before: current.Total(), After: next.Total(),
		Modifiers: toTerms(next.Modifiers), Cause: toTerm(cause),
	})
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	next.LogEntryID, err = c.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Round:       round,
		Kind:        actionlog.KindInitiativeCorrect,
		Payload:     payload,
		Snapshots:   []actionlog.Snapshot{snap},
	})
	if err != nil {
		return nil, fmt.Errorf("record %s: %w", actionlog.KindInitiativeCorrect, err)
	}
	if err := c.dal.SaveInitiative(ctx, characterID, &next); err != nil {
		return nil, fmt.Errorf("save initiative: %w", err)
	}
	return &next, nil
}

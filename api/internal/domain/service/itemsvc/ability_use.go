package itemsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// ReasonExhausted is why an ability cannot be used: its limit cannot pay a use
// (ADR 0049 step 5). A code the web words by the limit's kind ("épuisé jusqu'au
// repos long" for a per_day ability).
const ReasonExhausted = "exhausted"

// UseRefusal is why the ability cannot be used now, a reason code, "" when it
// can (ADR 0049 steps 5 and 14: the server serves `usable` and `reason` so the
// web derives nothing). Use does not depend on the item's state (step 7).
func (i Item) UseRefusal(a Ability) string {
	paid := false
	switch a.Limit.Kind {
	case LimitUnlimited:
		paid = true
	case LimitPerDay:
		paid = a.Limit.Left >= 1
	case LimitCharges:
		paid = a.Limit.Left >= max(a.Limit.Cost, 1)
	case LimitWithItem:
		paid = i.Count >= 1
	}
	if paid {
		return ""
	}
	return ReasonExhausted
}

// EffectResult is what one effect of an ability's use produced, in list
// order. A dice effect rolls its dice and applies them to nothing (ADR 0049
// step 9).
type EffectResult struct {
	Kind     EffectKind `json:"kind"`
	Dice     string     `json:"dice,omitempty"`
	Rolls    []int      `json:"rolls"`
	Modifier int        `json:"modifier,omitempty"`
	Total    int        `json:"total"`
	Save     string     `json:"save,omitempty"`
	Label    string     `json:"label,omitempty"`
}

// AbilityUseResult is what one use of an item's ability produced (ADR 0049
// step 7): the limit as it stands after the use, and each effect's result.
type AbilityUseResult struct {
	ItemID      string
	AbilityID   string
	Name        string         // the item's name
	AbilityName localized.Text // none: the item's name shows
	Type        string
	Limit       LimitKind
	// Left and Max are the counter of a per_day or charges ability after the
	// use (nil for the other kinds); Remaining is the number left to read:
	// that counter, or the quantity of a with_item ability, 0 for unlimited.
	Left      *int
	Max       *int
	Remaining int
	// Removed is true when this use spent the last potion or scroll: the item
	// is gone (popping the use brings it back).
	Removed    bool
	Effects    []EffectResult
	Notes      string
	LogEntryID int64
}

// UseAbility is ADR 0049 step 7: Use acts on one ability of an item, in one
// transaction. The limit is paid first (a refusal changes nothing:
// ErrAbilityExhausted, with ErrItemNotFound or ErrAbilityNotFound for an
// unknown item or ability); then the effects run in list order; then one
// item_use entry is written with the item's snapshot (popping it gives back
// the counter, or the item a with_item use removed); then the round's
// initiative is corrected as the last entry (step 15). The ability works
// whatever the item's state.
func (s *service) UseAbility(ctx context.Context, characterID int64, itemID, abilityID string) (*AbilityUseResult, error) {
	var res *AbilityUseResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.dal.Get(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		at := slices.IndexFunc(before.Abilities, func(a Ability) bool { return a.ID == abilityID })
		if at < 0 {
			return fmt.Errorf("%w: %s has no ability %q", ErrAbilityNotFound, before.Name.Any(), abilityID)
		}
		ability := before.Abilities[at]
		if reason := before.UseRefusal(ability); reason != "" {
			return fmt.Errorf("%w: %s (%s)", ErrAbilityExhausted, before.Name.Any(), abilityID)
		}
		after, err := s.payLimit(ctx, characterID, before, at)
		if err != nil {
			return err
		}
		r := &AbilityUseResult{
			ItemID: after.ID, AbilityID: ability.ID, Name: after.Name.Any(), AbilityName: ability.Name, Type: after.Type,
			Limit: ability.Limit.Kind, Removed: after.Removed(), Notes: ability.Note, Effects: []EffectResult{},
		}
		paid := after.Abilities[at].Limit
		switch ability.Limit.Kind {
		case LimitPerDay, LimitCharges:
			r.Left, r.Max, r.Remaining = &paid.Left, &paid.Max, paid.Left
		case LimitWithItem:
			r.Remaining = after.Count
		}
		for _, e := range ability.Effects {
			er, err := s.runEffect(e)
			if err != nil {
				return fmt.Errorf("item %s ability %s: %w", after.ID, ability.ID, err)
			}
			r.Effects = append(r.Effects, er)
		}
		var snapAfter *Item
		if !r.Removed {
			snapAfter = &after
		}
		snap, err := itemSnapshot(after.ID, &before, snapAfter)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(abilityRecord(r))
		if err != nil {
			return fmt.Errorf("encode item use payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindItemUse,
			Payload:     payload,
			Snapshots:   []actionlog.Snapshot{snap},
		})
		if err != nil {
			return fmt.Errorf("record item use: %w", err)
		}
		r.LogEntryID = id
		res = r
		return s.correctInitiative(ctx, characterID, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// payLimit writes the cost of one use of before's ability at index at (ADR
// 0049 step 5) and returns the item as it is now. per_day and charges move the
// ability's left (the item is written whole); with_item spends one of the
// quantity in one atomic statement, and a potion or a scroll at 0 is deleted;
// unlimited writes nothing.
func (s *service) payLimit(ctx context.Context, characterID int64, before Item, at int) (Item, error) {
	limit := before.Abilities[at].Limit
	switch limit.Kind {
	case LimitPerDay, LimitCharges:
		after := before
		after.Abilities = slices.Clone(before.Abilities)
		cost := 1
		if limit.Kind == LimitCharges {
			cost = max(limit.Cost, 1)
		}
		after.Abilities[at].Limit.Left = limit.Left - cost
		if err := s.dal.Update(ctx, characterID, after); err != nil {
			return Item{}, err
		}
		return after, nil
	case LimitWithItem:
		after, err := s.dal.Decrement(ctx, characterID, before.ID)
		if errors.Is(err, ErrItemEmpty) {
			return Item{}, fmt.Errorf("%w: %s", ErrAbilityExhausted, before.Name.Any())
		}
		if err != nil {
			return Item{}, err
		}
		after.Abilities = before.Abilities
		if after.Removed() {
			if err := s.dal.Delete(ctx, characterID, after.ID); err != nil {
				return Item{}, fmt.Errorf("remove spent item %s: %w", after.ID, err)
			}
		}
		return after, nil
	}
	return before, nil
}

// runEffect applies one effect of an ability (ADR 0049 step 12): a dice effect
// rolls its dice through the dice port and applies them to nothing.
func (s *service) runEffect(e Effect) (EffectResult, error) {
	rolls, mod, err := rollDice(s.dice, e.Dice)
	if err != nil {
		return EffectResult{}, err
	}
	total := mod
	for _, v := range rolls {
		total += v
	}
	return EffectResult{Kind: e.Kind, Dice: e.Dice, Rolls: rolls, Modifier: mod, Total: total, Save: e.Save, Label: e.Label}, nil
}

// abilityRecord is the use's log payload: the ability's fields, and the first
// dice effect and the counter again in the shape the journal words (useRecord).
func abilityRecord(r *AbilityUseResult) useRecord {
	rec := useRecord{
		ItemID: r.ItemID, AbilityID: r.AbilityID, Name: r.Name, AbilityName: r.AbilityName, Type: r.Type,
		Limit: string(r.Limit), Left: r.Left, Max: r.Max, Removed: r.Removed, Remaining: r.Remaining,
		Rolls: []int{}, Notes: r.Notes, Effects: r.Effects,
	}
	if r.Max != nil {
		rec.MaxCount = *r.Max
	}
	if i := slices.IndexFunc(r.Effects, func(e EffectResult) bool { return e.Kind == EffectDice }); i >= 0 {
		e := r.Effects[i]
		rec.Dice, rec.Rolls, rec.Modifier, rec.Total, rec.Save = e.Dice, e.Rolls, e.Modifier, e.Total, e.Save
	}
	return rec
}

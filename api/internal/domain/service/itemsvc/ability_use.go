package itemsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
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
	// Full is a heal that asked for the gap to max HP (no dice).
	Full bool `json:"full,omitempty"`
	// Applied is what the effect changed in the character (ADR 0049 step 7):
	// nil for a dice effect, which applies nothing.
	Applied *Applied `json:"applied,omitempty"`
	// Buff is the buff a buff effect put on the list (ADR 0049 step 12).
	Buff *AppliedBuff `json:"buff,omitempty"`
}

// AppliedBuff is the row a buff effect put on the buff list: its id and name,
// its rounds (0: untimed) and the modifiers it carries, in the buffs' effects.
type AppliedBuff struct {
	ID      string
	Name    string
	Rounds  int
	Effects buffsvc.BuffEffects
}

// MarshalJSON writes the modifiers under the buffs' wire keys (min_str,
// save_bonus, ac...), as an item's powers and effects store them.
func (a AppliedBuff) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID      string      `json:"id"`
		Name    string      `json:"name"`
		Rounds  int         `json:"rounds"`
		Effects jsonEffects `json:"effects"`
	}{a.ID, a.Name, a.Rounds, jsonEffects(a.Effects)})
}

// Applied is what one heal effect did to the HP ledger: the points it added
// (capped at max HP, ADR 0007 step 5) and the current HP around it.
type Applied struct {
	HPDelta  int `json:"hpDelta"`
	HPBefore int `json:"hpBefore"`
	HPAfter  int `json:"hpAfter"`
}

// hpRun follows the HP ledger across the heals of one use, read at the first
// heal: effects run in list order, so a second heal sees what the first filled.
type hpRun struct {
	read  bool
	total int // the ledger's sum of deltas, with this use's heals so far
	max   int // the character's max HP
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
// whatever the item's state. The read-then-write of a counter is atomic: InTx
// takes the one writer lock first (ADR 0025 step 4), so a concurrent use or
// edit of the item waits and then reads this one's result.
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
		hp := &hpRun{}
		hpDelta := 0
		var buffSnap *actionlog.Snapshot
		for _, e := range ability.Effects {
			er, snap, err := s.runEffect(ctx, characterID, hp, e, buffName(e, ability, after))
			if snap != nil {
				buffSnap = mergeSnapshots(buffSnap, *snap)
			}
			if err != nil {
				return fmt.Errorf("item %s ability %s: %w", after.ID, ability.ID, err)
			}
			r.Effects = append(r.Effects, er)
			if er.Applied != nil {
				hpDelta += er.Applied.HPDelta
			}
		}
		var snapAfter *Item
		if !r.Removed {
			snapAfter = &after
		}
		snap, err := itemSnapshot(after.ID, &before, snapAfter)
		if err != nil {
			return err
		}
		snaps := []actionlog.Snapshot{snap}
		if buffSnap != nil {
			// ADR 0049 step 12: the buffs' snapshot rides the use's entry, so
			// one undo pops the buff with the use.
			snaps = append(snaps, *buffSnap)
		}
		payload, err := json.Marshal(abilityRecord(r))
		if err != nil {
			return fmt.Errorf("encode item use payload: %w", err)
		}
		// ADR 0049 step 12: the heals are the entry's own HP delta, in the
		// use's transaction (one undo pops the HP with the counter); the
		// recording path caps it at max HP, as the effects did one by one.
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindItemUse,
			Payload:     payload,
			HPDelta:     hpDelta,
			Snapshots:   snaps,
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
// rolls its dice through the dice port and applies them to nothing; a heal
// rolls its dice (or asks for the gap to max HP) and reports the points the HP
// ledger takes, which the use's entry then records as its HP delta.
func (s *service) runEffect(ctx context.Context, characterID int64, hp *hpRun, e Effect, buffName string) (EffectResult, *actionlog.Snapshot, error) {
	res := EffectResult{Kind: e.Kind, Dice: e.Dice, Rolls: []int{}, Save: e.Save, Label: e.Label, Full: e.Full}
	if e.Kind == EffectBuff {
		return s.runBuff(ctx, characterID, e, buffName)
	}
	if e.Dice != "" {
		rolls, mod, err := rollDice(s.dice, e.Dice)
		if err != nil {
			return EffectResult{}, nil, err
		}
		res.Rolls, res.Modifier, res.Total = rolls, mod, mod
		for _, v := range rolls {
			res.Total += v
		}
	}
	if e.Kind != EffectHeal {
		return res, nil, nil
	}
	if !hp.read {
		total, err := s.log.HPTotal(ctx, characterID)
		if err != nil {
			return EffectResult{}, nil, fmt.Errorf("hp total: %w", err)
		}
		c, err := s.chars.View(ctx, characterID)
		if err != nil {
			return EffectResult{}, nil, fmt.Errorf("character: %w", err)
		}
		hp.read, hp.total, hp.max = true, total, c.HPMax
	}
	// A heal never harms: a negative roll (1d4-5) heals 0. Full is the gap to
	// max HP the ledger holds now (ADR 0008 step 17.4, as a spell's).
	requested := max(0, res.Total)
	if e.Full {
		requested = max(0, -hp.total)
		res.Total = requested
	}
	applied := actionlog.CapHeal(hp.total, requested)
	before := actionlog.CurrentHP(hp.max, hp.total)
	hp.total += applied
	res.Total = requested
	res.Applied = &Applied{HPDelta: applied, HPBefore: before, HPAfter: actionlog.CurrentHP(hp.max, hp.total)}
	return res, nil, nil
}

// buffName is the name a buff effect's row takes: its own, else its ability's,
// else the item's (ADR 0049 step 10).
func buffName(e Effect, a Ability, it Item) string {
	if e.Name != "" {
		return e.Name
	}
	if n := a.Name.Any(); n != "" {
		return n
	}
	return it.Name.Any()
}

// runBuff puts the effect's buff on the list through buffsvc.Add, the path a
// cast and a hand-made buff use, in the use's transaction (ADR 0049 step 12):
// a buff of the same name renews, a Strength one stacks, as ADR 0004 steps 6
// and 7 say. The snapshot it returns is the use's entry's.
func (s *service) runBuff(ctx context.Context, characterID int64, e Effect, name string) (EffectResult, *actionlog.Snapshot, error) {
	res := EffectResult{Kind: e.Kind, Rolls: []int{}}
	if s.buffs == nil {
		return EffectResult{}, nil, fmt.Errorf("a buff effect needs the buff service")
	}
	added, snap, err := s.buffs.Add(ctx, characterID, buffsvc.BuffEffect{Name: name, Duration: e.Rounds, Effects: e.Buff})
	if err != nil {
		return EffectResult{}, nil, fmt.Errorf("add buff: %w", err)
	}
	res.Buff = &AppliedBuff{ID: added.ID, Name: added.Name, Rounds: added.Duration, Effects: added.Effects}
	return res, &snap, nil
}

// mergeSnapshots joins two snapshots of one state key recorded by two buff
// effects of one use: the state before the first and after the last (an entry
// holds one snapshot per key).
func mergeSnapshots(first *actionlog.Snapshot, next actionlog.Snapshot) *actionlog.Snapshot {
	if first == nil {
		return &next
	}
	return &actionlog.Snapshot{Key: first.Key, Before: first.Before, After: next.After}
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

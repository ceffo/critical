package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// UseResult is what one use of an item produced: the dice rolled (none for an
// item without effect dice, like the Power Gem's Dispel) and the count left.
type UseResult struct {
	ItemID    string
	Name      string
	Type      string
	Behaviour Behaviour
	Remaining int
	MaxCount  int
	// Removed is true when this use spent the last charge of a charges item,
	// which is gone from the inventory (popping the use brings it back).
	Removed bool
	// Dice is the effect dice ("6d6"), empty when the item rolls nothing.
	Dice string
	// Rolls are the individual dice, in order, Modifier the flat "+K" of
	// the dice ("1d8+2"), and Total their sum.
	Rolls    []int
	Modifier int
	Total    int
	// Save is the save category the effect allows ("rod_staff_wand"), empty if none.
	Save       string
	Notes      string
	LogEntryID int64
}

// useRecord is itemsvc's JSON encoding of a use inside its action-log entry
// (payload), the same camelCase names as the /api/items/{id}/use response.
// Entries already in the log must stay decodable: add fields, never rename.
// Kind is the item kind of entries logged before the one item model.
type useRecord struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Type      string `json:"type"`
	Behaviour string `json:"behaviour"`
	Removed   bool   `json:"removed"`
	Remaining int    `json:"remaining"`
	MaxCount  int    `json:"maxCount"`
	Dice      string `json:"dice,omitempty"`
	Rolls     []int  `json:"rolls"`
	Total     int    `json:"total"`
	Save      string `json:"save,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Modifier  int    `json:"modifier,omitempty"`
	// Uses logged before the UI built it also carry an English "breakdown",
	// no longer read.
}

// UseItem spends one count of any item (the inventory's −), rolls the effect
// dice and logs the use. Dice are rolled after the decrement, inside the same
// transaction: any failure leaves the count (and a charges item at 0)
// untouched.
func (s *service) UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error) {
	var res *UseResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		item, err := s.Use(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		r := &UseResult{
			ItemID: item.ID, Name: item.Name.Any(), Type: item.Type, Behaviour: item.Behaviour,
			Remaining: item.Count, MaxCount: item.MaxCount, Removed: item.Removed(),
			Save: item.EffectSave, Notes: item.Notes, Rolls: []int{},
		}
		if item.EffectDice != "" {
			rolls, mod, err := rollDice(s.dice, item.EffectDice)
			if err != nil {
				return fmt.Errorf("item %s effect: %w", item.ID, err)
			}
			r.Dice, r.Rolls, r.Modifier = item.EffectDice, rolls, mod
			r.Total = mod
			for _, v := range rolls {
				r.Total += v
			}
		}
		snap, err := UseSnapshot(item)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(useRecord{
			ItemID: r.ItemID, Name: r.Name, Type: r.Type, Behaviour: string(r.Behaviour),
			Removed: r.Removed, Remaining: r.Remaining,
			MaxCount: r.MaxCount, Dice: r.Dice, Rolls: r.Rolls, Total: r.Total,
			Save: r.Save, Notes: r.Notes, Modifier: r.Modifier,
		})
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
		return s.correctInitiative(ctx, characterID, item)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// correctInitiative is ADR 0035 item 6.2 for an item used: a magic piece
// makes the round's initiative the same d10 plus its magic kind's PHB Table
// 56 modifier. A scroll's row is the casting time of its spell: a scroll
// that names its spell adds that spell's casting time
// (buffsvc.ScrollInitiative); one that names none is not counted
// (buffsvc.ItemInitiative). It is the use transaction's last entry. Any other piece (ammunition, gear) corrects nothing, nor does a use
// before catalogue format 7 (no Table 56).
func (s *service) correctInitiative(ctx context.Context, characterID int64, item Item) error {
	if item.Category != CategoryMagic || s.initiativeItems == nil {
		return nil
	}
	row, ok := s.initiativeItems.Row(item.MagicKind)
	if !ok {
		return nil
	}
	cause := buffsvc.ItemInitiative(item.ID, item.Name.Any(), item.MagicKind, row.Modifier, row.HasModifier)
	if !row.HasModifier && item.SpellID != "" {
		var err error
		if cause, err = s.scrollInitiative(ctx, item); err != nil {
			return err
		}
	}
	return s.correctBy(ctx, characterID, cause)
}

// scrollInitiative is the term of a scroll that names its spell: the
// catalogue's casting time of that spell (a spell it no longer knows, or no
// catalogue, reads as special).
func (s *service) scrollInitiative(ctx context.Context, item Item) (buffsvc.InitiativeModifier, error) {
	var ct spellstats.CastingTime
	found := false
	if s.spells != nil {
		spell, ok, err := s.spells.Describe(ctx, item.SpellID)
		if err != nil {
			return buffsvc.InitiativeModifier{}, fmt.Errorf("describe %s: %w", item.SpellID, err)
		}
		ct, found = spell.Stats.CastingTime, ok
	}
	return buffsvc.ScrollInitiative(item.ID, item.Name.Any(), item.MagicKind, item.SpellID, ct, found), nil
}

// correctBy corrects the round's initiative by cause.
func (s *service) correctBy(ctx context.Context, characterID int64, cause buffsvc.InitiativeModifier) error {
	if _, err := s.initiative.CorrectInitiative(ctx, characterID, cause); err != nil {
		return fmt.Errorf("correct initiative: %w", err)
	}
	return nil
}

// rollDice rolls "NdM" or "NdM+K" through the dice port: the dice in order
// and the flat modifier K (0 without one).
func rollDice(d Dice, dice string) (rolls []int, mod int, err error) {
	n, sides, mod, err := ParseDice(dice)
	if err != nil {
		return nil, 0, err
	}
	rolls = make([]int, n)
	for i := range rolls {
		rolls[i] = d.RollDie(sides)
	}
	return rolls, mod, nil
}

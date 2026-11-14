package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// UseResult is what spending one of an item's stock produced (the inventory's
// minus, ADR 0049 step 2): the count left. It rolls nothing and applies no
// effect.
type UseResult struct {
	ItemID    string
	Name      string
	Type      string
	Remaining int
	// Removed is true when this spend took the last potion or scroll: the item
	// is gone from the inventory (popping the use brings it back).
	Removed    bool
	Notes      string
	LogEntryID int64
}

// useRecord is itemsvc's JSON encoding of a use inside its action-log entry
// (payload), the same camelCase names as the use endpoints' responses.
// Entries already in the log must stay decodable: add fields, never rename.
// Kind is the item kind of entries logged before the one item model. An entry
// without AbilityId is a spend-one (and every entry recorded before ADR 0049).
// Behaviour, Dice, Rolls, Total, Save and Modifier keep the shape the journal
// words: an ability's use fills the dice fields from its first dice effect and
// Remaining and MaxCount from its counter.
type useRecord struct {
	ItemID      string         `json:"itemId"`
	AbilityID   string         `json:"abilityId,omitempty"`
	Name        string         `json:"name"`
	AbilityName localized.Text `json:"abilityName,omitempty"`
	Kind        string         `json:"kind,omitempty"`
	Type        string         `json:"type"`
	Behaviour   string         `json:"behaviour,omitempty"`
	// Limit, Left and Max are the ability's limit kind and its counter after
	// the use (Left, Max only for per_day and charges).
	Limit     string         `json:"limit,omitempty"`
	Left      *int           `json:"left,omitempty"`
	Max       *int           `json:"max,omitempty"`
	Removed   bool           `json:"removed"`
	Remaining int            `json:"remaining"`
	MaxCount  int            `json:"maxCount"`
	Dice      string         `json:"dice,omitempty"`
	Rolls     []int          `json:"rolls"`
	Total     int            `json:"total"`
	Save      string         `json:"save,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	Modifier  int            `json:"modifier,omitempty"`
	Effects   []EffectResult `json:"effects,omitempty"`
	// Uses logged before the UI built it also carry an English "breakdown",
	// no longer read.
}

// UseItem spends one of a stock item's quantity (the inventory's minus, ADR
// 0049 step 2): a count change with no ability, no effect and no initiative,
// logged as an item_use entry without an abilityId. A potion or a scroll spent
// to 0 is removed in the same entry; undo puts it back. The count moves in one
// atomic statement, after the refusals are read: ErrNotUsable for an item with
// no stock (a weapon, a ring) or one with a per_day or charges ability (its
// counter is the ability's, Use acts on it), ErrItemEmpty at 0.
func (s *service) UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error) {
	var res *UseResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.dal.Get(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		if !before.Usable() || before.holdsPerObjectLimit() {
			return fmt.Errorf("%w: %s (%s) has no stock to spend", ErrNotUsable, before.Name.Any(), before.Category)
		}
		item, err := s.Use(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		r := &UseResult{
			ItemID: item.ID, Name: item.Name.Any(), Type: item.Type,
			Remaining: item.Count, Removed: item.Removed(), Notes: item.Notes,
		}
		snap, err := UseSnapshot(item)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(useRecord{
			ItemID: r.ItemID, Name: r.Name, Type: r.Type, Removed: r.Removed, Remaining: r.Remaining,
			Rolls: []int{}, Notes: r.Notes,
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
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// correctInitiative is ADR 0049 step 15 (ADR 0035 item 6.2) for an ability
// used: the round's initiative takes the d10 plus the item's PHB Table 56
// modifier by its magic kind. A scroll's row is the casting time of its spell:
// the spell is the one the used ability's first spell effect names (ADR 0049
// step 9: the scroll's spell lives in its effect), which adds that spell's
// casting time (buffsvc.ScrollInitiative); a scroll whose ability holds none is
// not counted
// (buffsvc.ItemInitiative). An item of any other category (the sword's "turn
// zombies", a rope) takes the miscellaneous row (the user's ruling Q3). It is
// the use transaction's last entry. Nothing is corrected before catalogue
// format 7 (no Table 56); out of combat the corrector itself does nothing.
func (s *service) correctInitiative(ctx context.Context, characterID int64, item Item, ability Ability) error {
	if s.initiativeItems == nil {
		return nil
	}
	kind := item.MagicKind
	if item.Category != CategoryMagic {
		kind = MagicMisc
	}
	row, ok := s.initiativeItems.Row(kind)
	if !ok {
		return nil
	}
	cause := buffsvc.ItemInitiative(item.ID, item.Name.Any(), kind, row.Modifier, row.HasModifier)
	if spellID := ability.spellID(); !row.HasModifier && item.Category == CategoryMagic && spellID != "" {
		var err error
		if cause, err = s.scrollInitiative(ctx, item, spellID); err != nil {
			return err
		}
	}
	return s.correctBy(ctx, characterID, cause)
}

// scrollInitiative is the term of a scroll whose ability casts spellID: the
// catalogue's casting time of that spell (a spell it no longer knows, or no
// catalogue, reads as special).
func (s *service) scrollInitiative(ctx context.Context, item Item, spellID string) (buffsvc.InitiativeModifier, error) {
	var ct spellstats.CastingTime
	found := false
	if s.spells != nil {
		spell, ok, err := s.spells.Describe(ctx, spellID)
		if err != nil {
			return buffsvc.InitiativeModifier{}, fmt.Errorf("describe %s: %w", spellID, err)
		}
		ct, found = spell.Stats.CastingTime, ok
	}
	return buffsvc.ScrollInitiative(item.ID, item.Name.Any(), item.MagicKind, spellID, ct, found), nil
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

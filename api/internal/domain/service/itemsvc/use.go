package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// UseResult is what one use of an item produced: the dice rolled with their
// breakdown (none for an item without effect dice, like the Power Gem's
// Dispel) and the count left.
type UseResult struct {
	ItemID    string
	Name      string
	Kind      Kind
	Remaining int
	MaxCount  int
	// Dice is the effect dice ("6d6"), empty when the item rolls nothing.
	Dice string
	// Rolls are the individual dice, in order; Total is their sum.
	Rolls []int
	Total int
	// Save is the save category the effect allows ("rod_staff_wand"), empty if none.
	Save  string
	Notes string
	// Breakdown is the roll with the numbers filled in, e.g. "6d6: 3 + 4 + ... = 21".
	Breakdown  string
	LogEntryID int64
}

// useRecord is itemsvc's JSON encoding of a use inside its action-log entry
// (payload), the same camelCase names as the /api/items/{id}/use response.
// Entries already in the log must stay decodable: add fields, never rename.
type useRecord struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Remaining int    `json:"remaining"`
	MaxCount  int    `json:"maxCount"`
	Dice      string `json:"dice,omitempty"`
	Rolls     []int  `json:"rolls"`
	Total     int    `json:"total"`
	Save      string `json:"save,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Breakdown string `json:"breakdown,omitempty"`
}

// UseItem spends one count, rolls the effect dice and logs the use. Dice
// are rolled after the decrement, inside the same transaction: any failure
// leaves the count untouched.
func (s *service) UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error) {
	var res *UseResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		item, err := s.dal.Decrement(ctx, itemID)
		if err != nil {
			return err
		}
		if item.Kind == KindAmmo {
			// Rolled back with the transaction: the arrow is not spent.
			return fmt.Errorf("%w: %s is fired by attacks, not used", domain.ErrInvalidInput, item.Name)
		}
		r := &UseResult{
			ItemID: item.ID, Name: item.Name, Kind: item.Kind,
			Remaining: item.Count, MaxCount: item.MaxCount,
			Save: item.EffectSave, Notes: item.Notes, Rolls: []int{},
		}
		if item.EffectDice != "" {
			rolls, err := rollDice(s.dice, item.EffectDice)
			if err != nil {
				return fmt.Errorf("item %s effect: %w", item.ID, err)
			}
			r.Dice, r.Rolls = item.EffectDice, rolls
			for _, v := range rolls {
				r.Total += v
			}
			r.Breakdown = breakdown(r)
		}
		snap, err := UseSnapshot(item)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(useRecord{
			ItemID: r.ItemID, Name: r.Name, Kind: string(r.Kind), Remaining: r.Remaining,
			MaxCount: r.MaxCount, Dice: r.Dice, Rolls: r.Rolls, Total: r.Total,
			Save: r.Save, Notes: r.Notes, Breakdown: r.Breakdown,
		})
		if err != nil {
			return fmt.Errorf("encode item use payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindItemUse,
			Summary:     summary(r),
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

// rollDice rolls "NdM" (case-insensitive) through the dice port and returns
// every die.
func rollDice(d Dice, dice string) ([]int, error) {
	n, m, ok := strings.Cut(strings.ToLower(strings.TrimSpace(dice)), "d")
	num, err1 := strconv.Atoi(n)
	sides, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || num < 1 || sides < 1 {
		return nil, fmt.Errorf("invalid effect dice %q", dice)
	}
	rolls := make([]int, num)
	for i := range rolls {
		rolls[i] = d.RollDie(sides)
	}
	return rolls, nil
}

func breakdown(r *UseResult) string {
	parts := make([]string, len(r.Rolls))
	for i, v := range r.Rolls {
		parts[i] = strconv.Itoa(v)
	}
	return fmt.Sprintf("%s: %s = %d", r.Dice, strings.Join(parts, " + "), r.Total)
}

// summary is the English log line (the UI renders the payload instead).
func summary(r *UseResult) string {
	if r.Dice == "" {
		return fmt.Sprintf("Used %s (%d left)", r.Name, r.Remaining)
	}
	return fmt.Sprintf("Used %s: %s = %d (%d left)", r.Name, r.Dice, r.Total, r.Remaining)
}

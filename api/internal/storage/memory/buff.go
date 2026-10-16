package memory

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// Buffs is buffsvc.DAL over the store, plus the round read the action log
// stamps entries with: the active buffs, the round counter and its
// initiative, the saved buff templates.
type Buffs struct{ s *Store }

var _ buffsvc.DAL = Buffs{}

// Buffs returns the buff port.
func (s *Store) Buffs() Buffs { return Buffs{s} }

// GetActiveBuffs returns the character's active buffs (empty when none).
func (b Buffs) GetActiveBuffs(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error) {
	out := []buffsvc.BuffEffect{}
	err := b.s.read(ctx, func(t *tables) error {
		if buffs := t.buffs[characterID]; len(buffs) > 0 {
			out = clone(buffs)
		}
		return nil
	})
	return out, err
}

// SaveActiveBuffs replaces the character's active buffs.
func (b Buffs) SaveActiveBuffs(ctx context.Context, characterID int64, buffs []buffsvc.BuffEffect) error {
	return b.s.write(ctx, func(t *tables) error {
		t.buffs[characterID] = clone(buffs)
		return nil
	})
}

// GetRoundNumber returns the round counter; 0 when none was stored.
func (b Buffs) GetRoundNumber(ctx context.Context, characterID int64) (int, error) {
	var round int
	err := b.s.read(ctx, func(t *tables) error {
		round = t.rounds[characterID].round
		return nil
	})
	return round, err
}

// SaveRoundNumber stores the round counter.
func (b Buffs) SaveRoundNumber(ctx context.Context, characterID int64, round int) error {
	return b.s.write(ctx, func(t *tables) error {
		r := t.rounds[characterID]
		r.round = round
		t.rounds[characterID] = r
		return nil
	})
}

// GetInitiative returns the round's initiative; nil when there is none.
func (b Buffs) GetInitiative(ctx context.Context, characterID int64) (*buffsvc.Initiative, error) {
	var out *buffsvc.Initiative
	err := b.s.read(ctx, func(t *tables) error {
		out = clone(t.rounds[characterID].initiative)
		return nil
	})
	return out, err
}

// SaveInitiative stores the round's initiative; nil clears it.
func (b Buffs) SaveInitiative(ctx context.Context, characterID int64, initiative *buffsvc.Initiative) error {
	return b.s.write(ctx, func(t *tables) error {
		r := t.rounds[characterID]
		r.initiative = clone(initiative)
		t.rounds[characterID] = r
		return nil
	})
}

// ListSavedBuffs returns the character's templates, oldest first.
func (b Buffs) ListSavedBuffs(ctx context.Context, characterID int64) ([]buffsvc.SavedBuff, error) {
	out := []buffsvc.SavedBuff{}
	err := b.s.read(ctx, func(t *tables) error {
		for _, sb := range t.savedBuffs {
			if sb.characterID == characterID {
				out = append(out, clone(sb.SavedBuff))
			}
		}
		slices.SortFunc(out, func(a, b buffsvc.SavedBuff) int { return cmp.Compare(a.ID, b.ID) })
		return nil
	})
	return out, err
}

// ReplaceSavedBuffs makes the character's templates equal to saved: a
// template with an id is restored under it with its creation time (now when
// it has none), one without gets a new id.
func (b Buffs) ReplaceSavedBuffs(ctx context.Context, characterID int64, saved []buffsvc.SavedBuff) error {
	return b.s.write(ctx, func(t *tables) error {
		for id, sb := range t.savedBuffs {
			if sb.characterID == characterID {
				delete(t.savedBuffs, id)
			}
		}
		for _, sb := range saved {
			row := savedBuff{characterID: characterID, SavedBuff: clone(sb)}
			if sb.ID != 0 {
				if _, taken := t.savedBuffs[sb.ID]; taken {
					return errDuplicate("saved_buffs", sb.ID)
				}
				takeID(&t.savedBuffSeq, sb.ID)
				if row.CreatedAt.IsZero() {
					row.CreatedAt = now()
				}
				row.CreatedAt = row.CreatedAt.UTC().Truncate(time.Second)
			} else {
				row.ID = nextID(&t.savedBuffSeq)
				row.CreatedAt = now()
			}
			t.savedBuffs[row.ID] = row
		}
		return nil
	})
}

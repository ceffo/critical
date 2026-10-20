package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// PicksStateKey is the action-log state key of a character's proficiency picks.
const PicksStateKey = "proficiencies"

// Picks writes a character's proficiency picks (characters.proficiency_picks). It joins the ctx
// transaction.
type Picks interface {
	SetProficiencyPicks(ctx context.Context, characterID int64, picks charactersvc.Picks) error
}

// picksSnaps is the snapshotter of the picks (ADR 0018 step 8).
var picksSnaps = actionlog.NewSnapshotter[picksState](PicksStateKey)

// pickState is one pick in a snapshot or a payload.
type pickState struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// weaponPickState is one weapon pick in a snapshot or a payload.
type weaponPickState struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

// picksState is the JSON encoding of a "proficiencies" snapshot, and the before and after of a
// proficiencies entry: the picks by pool, in the player's order. Entries already in the log must
// stay decodable: add fields (the languages), never rename. Weapon is absent from the entries
// logged before the weapon pool (nil): restoring one of them leaves the weapon picks as stored.
type picksState struct {
	Nonweapon []pickState        `json:"nonweapon"`
	Weapon    *[]weaponPickState `json:"weapon,omitempty"`
}

// picksRecord is the payload of a proficiencies entry, the picks before and after the save. The
// UI words it. Add fields, never rename.
type picksRecord struct {
	Before picksState `json:"before"`
	After  picksState `json:"after"`
}

func toState(p charactersvc.Picks) picksState {
	out := picksState{Nonweapon: make([]pickState, 0, len(p.Nonweapon))}
	for _, x := range p.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, pickState(x))
	}
	weapon := make([]weaponPickState, 0, len(p.Weapon))
	for _, x := range p.Weapon {
		weapon = append(weapon, weaponPickState(x))
	}
	out.Weapon = &weapon
	return out
}

// picks are the snapshot's picks; Weapon is nil for an entry logged before the weapon pool (the
// stored weapon picks stay).
func (s picksState) picks() charactersvc.Picks {
	out := charactersvc.Picks{Nonweapon: make([]charactersvc.Pick, 0, len(s.Nonweapon))}
	for _, x := range s.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, charactersvc.Pick(x))
	}
	if s.Weapon != nil {
		out.Weapon = make([]charactersvc.WeaponPick, 0, len(*s.Weapon))
		for _, x := range *s.Weapon {
			out.Weapon = append(out.Weapon, charactersvc.WeaponPick(x))
		}
	}
	return out
}

// PreviewProficiencies is the character as it would read with picks as its proficiency picks
// (ADR 0037 step 12): the pool, each pick's figures and the warnings, refused as
// SetProficiencies refuses. It writes and logs nothing.
func (s *service) PreviewProficiencies(ctx context.Context, characterID int64, picks charactersvc.Picks) (*charactersvc.Character, error) {
	var out *charactersvc.Character
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.chars.PreviewProficiencies(ctx, characterID, picks)
		return err
	})
	return out, err
}

// SetProficiencies is ADR 0037 steps 12 and 13: the player saves the whole list of picks of a
// pool or of both (a nil pool is left as stored), checked by charactersvc.ValidPicks against the
// stored ones; at the realized level a save
// that leaves the pool over budget and spends more is refused (step 10,
// charactersvc.PicksWithinBudget), an existing overspend that does not grow stays a warning. It
// is one proficiencies entry of the action log with the picks' snapshot, in one transaction, so
// undo gives them back; the same picks as stored record nothing; a refusal writes nothing.
// Returns the character as read after the save.
func (s *service) SetProficiencies(ctx context.Context, characterID int64, picks charactersvc.Picks) (*charactersvc.Character, error) {
	var out *charactersvc.Character
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		before := c.Proficiencies.Clone()
		next, err := charactersvc.ValidPicks(s.book, c.Class, picks, before)
		if err != nil {
			return err
		}
		// ADR 0037 step 10: over budget at the realized level and spending
		// more is refused before any write or log entry.
		// The languages are unchanged by a picks save but spend from the same pool (step 6).
		langs := c.LanguageSlots()
		if err := charactersvc.PicksWithinBudget(s.book, c.Class, c.Derived.RealizedLevel, c.Int,
			charactersvc.Spending{Picks: before, Languages: langs}, charactersvc.Spending{Picks: next, Languages: langs}); err != nil {
			return err
		}
		if !before.Equal(next) {
			if err := s.picks.SetProficiencyPicks(ctx, characterID, next); err != nil {
				return fmt.Errorf("set proficiency picks: %w", err)
			}
			b, a := toState(before), toState(next)
			payload, err := json.Marshal(picksRecord{Before: b, After: a})
			if err != nil {
				return fmt.Errorf("encode proficiencies payload: %w", err)
			}
			snap, err := picksSnaps.Snapshot("", &b, &a)
			if err != nil {
				return err
			}
			if _, err := s.log.Record(ctx, actionlog.NewEntry{
				CharacterID: characterID, Kind: actionlog.KindProficiencies, Payload: payload,
				Snapshots: []actionlog.Snapshot{snap},
			}); err != nil {
				return fmt.Errorf("record proficiencies: %w", err)
			}
		}
		out, err = s.chars.GetCharacter(ctx, characterID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// restorePicks writes a snapshot's picks back: undo the save's "before", redo its "after" (ADR
// 0019), inside the undo's transaction. A snapshot logged before the weapon pool carries no
// weapon picks: the stored ones stay.
func (s *service) restorePicks(ctx context.Context, characterID int64, _ string, st picksState) error {
	picks := st.picks()
	if picks.Weapon == nil {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		picks.Weapon = c.Proficiencies.Clone().Weapon
	}
	if err := s.picks.SetProficiencyPicks(ctx, characterID, picks); err != nil {
		return fmt.Errorf("restore proficiency picks: %w", err)
	}
	return nil
}

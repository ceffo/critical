package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// PicksStateKey is the action-log state key of a character's proficiency picks and languages.
const PicksStateKey = "proficiencies"

// Picks writes a character's proficiency picks (characters.proficiency_picks) and its languages
// (characters.languages, ADR 0037 step 18). Both join the ctx transaction.
type Picks interface {
	SetProficiencyPicks(ctx context.Context, characterID int64, picks skills.Picks) error
	SetLanguages(ctx context.Context, characterID int64, languages []string) error
}

// Edit is a save of the Skills tab's editor (ADR 0037 steps 12 and 18): the picks by pool (a nil
// pool is left as stored) and the languages (nil: left as stored).
type Edit struct {
	Picks     skills.Picks
	Languages []string
}

// Proficiencies are the pools as a preview or a save of the editor reads them (ADR 0037 steps 3
// to 6 and 11): each pool with its terms and each pick's figures (nil without the catalogue's
// Table 34), and the slot warnings.
type Proficiencies struct {
	Slots    *skills.ProficiencySlots
	Warnings []skills.Warning
}

// picksSnaps is the snapshotter of the picks and the languages (ADR 0018 step 8).
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
// proficiencies entry: the picks by pool, in the player's order, and the languages. Entries
// already in the log must stay decodable: add fields, never rename. Weapon is absent from the
// entries logged before the weapon pool, Languages from those logged before the languages
// moved to the Skills tab (ADR 0037 step 18): restoring one of them leaves the stored weapon
// picks or languages.
type picksState struct {
	Nonweapon []pickState        `json:"nonweapon"`
	Weapon    *[]weaponPickState `json:"weapon,omitempty"`
	Languages *[]string          `json:"languages,omitempty"`
}

// picksRecord is the payload of a proficiencies entry, the picks and the languages before and
// after the save. The UI words it. Add fields, never rename.
type picksRecord struct {
	Before picksState `json:"before"`
	After  picksState `json:"after"`
}

func toState(p skills.Picks, languages []string) picksState {
	out := picksState{Nonweapon: make([]pickState, 0, len(p.Nonweapon))}
	for _, x := range p.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, pickState(x))
	}
	weapon := make([]weaponPickState, 0, len(p.Weapon))
	for _, x := range p.Weapon {
		weapon = append(weapon, weaponPickState(x))
	}
	out.Weapon = &weapon
	langs := append([]string{}, languages...)
	out.Languages = &langs
	return out
}

// picks are the snapshot's picks; Weapon is nil for an entry logged before the weapon pool (the
// stored weapon picks stay).
func (s picksState) picks() skills.Picks {
	out := skills.Picks{Nonweapon: make([]skills.Pick, 0, len(s.Nonweapon))}
	for _, x := range s.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, skills.Pick(x))
	}
	if s.Weapon != nil {
		out.Weapon = make([]skills.WeaponPick, 0, len(*s.Weapon))
		for _, x := range *s.Weapon {
			out.Weapon = append(out.Weapon, skills.WeaponPick(x))
		}
	}
	return out
}

// edited is the state an edit of c gives (ADR 0037 steps 6, 10, 12 and 18): the picks checked by
// skills.ValidPicks against the stored ones, the languages by skills.ValidLanguages (nil: as
// read), then the budget at the realized level, each pool judged on its own and the languages in
// the non-weapon one; nothing is written. A refusal is a *domain.FieldError or a
// *domain.BudgetError.
func (s *service) edited(c *charactersvc.Character, e Edit) (skills.Picks, []string, error) {
	next, err := skills.ValidPicks(s.book, c.Class, e.Picks, c.Proficiencies)
	if err != nil {
		return skills.Picks{}, nil, err
	}
	langs := c.Details.Languages
	if e.Languages != nil {
		if langs, err = skills.ValidLanguages(s.languages, c.Race, c.Details.Languages, e.Languages); err != nil {
			return skills.Picks{}, nil, err
		}
	}
	native := s.languages.Native(c.Race)
	before := skills.Sheet{Picks: c.Proficiencies, Languages: c.Details.Languages, Native: native}
	after := skills.Sheet{Picks: next, Languages: langs, Native: native}
	if err := skills.PicksWithinBudget(s.book, c.Class, c.Derived.RealizedLevel, c.Int, before.Spending(), after.Spending()); err != nil {
		return skills.Picks{}, nil, err
	}
	return next, langs, nil
}

// proficiencies reads the pools of c with picks and languages (skills.Read, the sheet's own
// assembly).
func (s *service) proficiencies(c *charactersvc.Character, picks skills.Picks, languages []string) *Proficiencies {
	f, ws := skills.Read(s.book, skills.Sheet{
		Class: c.Class, Level: c.Derived.RealizedLevel, Intelligence: c.Int, Picks: picks,
		Languages: languages, Native: s.languages.Native(c.Race),
		Score: func(a string) (int, bool) { return c.Score(charactersvc.Ability(a)) },
	})
	return &Proficiencies{Slots: f, Warnings: ws}
}

// PreviewProficiencies is the pools as they would read after the edit (ADR 0037 step 12): each
// pool, each pick's figures and the warnings, refused as SetProficiencies refuses. It writes and
// logs nothing.
func (s *service) PreviewProficiencies(ctx context.Context, characterID int64, e Edit) (*Proficiencies, error) {
	var out *Proficiencies
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		next, langs, err := s.edited(c, e)
		if err != nil {
			return err
		}
		out = s.proficiencies(c, next, langs)
		return nil
	})
	return out, err
}

// SetProficiencies is ADR 0037 steps 12, 13 and 18: the player saves the whole list of picks of
// a pool or of both (a nil pool is left as stored) and the languages (nil: left as stored),
// checked against the stored ones; at the realized level a save that leaves a pool over budget
// and spends more is refused (step 10), an existing overspend that does not grow stays a
// warning. It is one proficiencies entry of the action log with the picks' and the languages'
// snapshot, in one transaction, so one undo gives both back; the same picks and languages as
// stored (in any order) record nothing; a refusal writes nothing. Returns the pools as read
// after the save.
func (s *service) SetProficiencies(ctx context.Context, characterID int64, e Edit) (*Proficiencies, error) {
	var out *Proficiencies
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		next, langs, err := s.edited(c, e)
		if err != nil {
			return err
		}
		before, beforeLangs := c.Proficiencies.Clone(), c.Details.Languages
		picksChanged, langsChanged := !before.Equal(next), !skills.LanguagesEqual(beforeLangs, langs)
		if picksChanged || langsChanged {
			if picksChanged {
				if err := s.picks.SetProficiencyPicks(ctx, characterID, next); err != nil {
					return fmt.Errorf("set proficiency picks: %w", err)
				}
			}
			if langsChanged {
				if err := s.picks.SetLanguages(ctx, characterID, langs); err != nil {
					return fmt.Errorf("set languages: %w", err)
				}
			}
			b, a := toState(before, beforeLangs), toState(next, langs)
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
		out = s.proficiencies(c, next, langs)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// restorePicks writes a snapshot's picks and languages back: undo the save's "before", redo its
// "after" (ADR 0019), inside the undo's transaction. A snapshot logged before the weapon pool
// carries no weapon picks, one logged before the languages moved to the Skills tab no
// languages: the stored ones stay.
func (s *service) restorePicks(ctx context.Context, characterID int64, _ string, st picksState) error {
	var stored *charactersvc.Character
	load := func() error {
		if stored != nil {
			return nil
		}
		c, err := s.chars.GetCharacter(ctx, characterID)
		stored = c
		return err
	}
	picks := st.picks()
	if picks.Weapon == nil {
		if err := load(); err != nil {
			return err
		}
		picks.Weapon = stored.Proficiencies.Clone().Weapon
	}
	if err := s.picks.SetProficiencyPicks(ctx, characterID, picks); err != nil {
		return fmt.Errorf("restore proficiency picks: %w", err)
	}
	// Only when they differ from the stored ones: a picks-only entry has the same languages on
	// both sides and must not rewrite the column (a hidden free-text word survives, ADR 0032
	// step 8.1).
	if st.Languages != nil {
		if err := load(); err != nil {
			return err
		}
	}
	if st.Languages != nil && !skills.LanguagesEqual(stored.Details.Languages, *st.Languages) {
		if err := s.picks.SetLanguages(ctx, characterID, *st.Languages); err != nil {
			return fmt.Errorf("restore languages: %w", err)
		}
	}
	return nil
}

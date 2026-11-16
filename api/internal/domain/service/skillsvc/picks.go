package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
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

// weaponPickState is one weapon pick in a snapshot or a payload: a catalogue weapon code (ADR
// 0037 step 21.7).
type weaponPickState struct {
	Code  string `json:"code"`
	Slots int    `json:"slots"`
}

// typePickState is one weapon type pick of an entry logged before a pick named a weapon (ADR 0037
// step 7 before step 21), and one type pick left to redo (step 21.8.3).
type typePickState struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

// picksState is the JSON encoding of a "proficiencies" snapshot, and the before and after of a
// proficiencies entry: the picks by pool, in the player's order, the type picks left to redo
// and the languages. Entries already in the log must stay decodable: add fields, never rename.
// Weapons (codes, ADR 0037 step 21.7) is absent from the entries logged before a pick named a
// weapon, which carry Weapon (types) or, before the weapon pool, nothing; Languages is absent
// from those logged before the languages moved to the Skills tab (step 18): restoring one of
// them leaves the stored weapon picks (and the redo list) or languages.
type picksState struct {
	Nonweapon []pickState `json:"nonweapon"`
	// Weapon is an older entry's type picks, read for nothing (step 21.7): the journal words it.
	Weapon     *[]typePickState   `json:"weapon,omitempty"`
	Weapons    *[]weaponPickState `json:"weapons,omitempty"`
	WeaponRedo *[]typePickState   `json:"weaponRedo,omitempty"`
	Languages  *[]string          `json:"languages,omitempty"`
	// FreeGrants is true on every entry logged since a class's free proficiencies cost 0 (ADR
	// 0037 step 22.7): an entry without it stores a free pick at the cost it paid then, which its
	// undo or redo gives back (skills.ReturnFreeSlots), as the data step did for the stored picks.
	FreeGrants bool `json:"freeGrants,omitempty"`
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
	out.Weapons = &weapon
	redo := make([]typePickState, 0, len(p.WeaponRedo))
	for _, x := range p.WeaponRedo {
		redo = append(redo, typePickState(x))
	}
	out.WeaponRedo = &redo
	langs := append([]string{}, languages...)
	out.Languages = &langs
	out.FreeGrants = true
	return out
}

// picks are the snapshot's picks; Weapon is nil for an entry logged before a pick named a weapon
// (ADR 0037 step 21.7: the stored weapon picks and redo list stay).
func (s picksState) picks() skills.Picks {
	out := skills.Picks{Nonweapon: make([]skills.Pick, 0, len(s.Nonweapon))}
	for _, x := range s.Nonweapon {
		out.Nonweapon = append(out.Nonweapon, skills.Pick(x))
	}
	if s.Weapons != nil {
		out.Weapon = make([]skills.WeaponPick, 0, len(*s.Weapons))
		for _, x := range *s.Weapons {
			out.Weapon = append(out.Weapon, skills.WeaponPick(x))
		}
		if s.WeaponRedo != nil {
			for _, x := range *s.WeaponRedo {
				out.WeaponRedo = append(out.WeaponRedo, skills.WeaponRedo(x))
			}
		}
	}
	return out
}

// edited is the state an edit of c gives (ADR 0037 steps 6, 10, 12 and 18): skills.Apply on the
// stored sheet at the realized level (the picks, the languages, then the budget); nothing is
// written. A refusal is a *domain.FieldError or a *domain.BudgetError. The weapon picks are
// checked under the ruleset's weapon lift (ADR 0041 step 2.5).
func (s *service) edited(c charactersvc.View, e Edit, lift ruleset.WeaponLift) (skills.Picks, []string, error) {
	before := skills.Sheet{
		Class: c.Primary().Class, Level: c.Primary().RealizedLevel, Classes: c.SkillClasses(), Intelligence: c.Int,
		Picks: c.ProficiencyPicks, Languages: c.Languages, Native: s.languages.Native(c.Race),
		MulticlassAnyWeapon: lift.MulticlassAnyWeapon,
	}
	after, err := skills.Apply(s.book, s.languages, c.Race, before, skills.Edit(e))
	if err != nil {
		return skills.Picks{}, nil, err
	}
	return after.Picks, after.Languages, nil
}

// proficiencies reads the pools of c with picks and languages (skills.Read, the sheet's own
// assembly) under the ruleset's weapon lift. Each pick's check target is built from the score in
// effect (ADR 0014 step 1 as amended), as the Skills tab and the roll build it, so the editor shows
// the target the check rolls against.
func (s *service) proficiencies(ctx context.Context, characterID int64, c charactersvc.View, picks skills.Picks, languages []string, lift ruleset.WeaponLift) (*Proficiencies, error) {
	scores := s.scoresInEffect(ctx, characterID)
	var failed error
	f, ws := skills.Read(s.book, skills.Sheet{
		Class: c.Primary().Class, Level: c.Primary().RealizedLevel, Classes: c.SkillClasses(), Intelligence: c.Int, Picks: picks,
		Languages: languages, Native: s.languages.Native(c.Race), MulticlassAnyWeapon: lift.MulticlassAnyWeapon,
		Score: func(a string) (int, bool) {
			if _, ok := c.Score(charactersvc.Ability(a)); !ok || failed != nil {
				return 0, false
			}
			got, err := scores(Ability(a))
			if err != nil {
				failed = fmt.Errorf("proficiency target, ability %q: %w", a, err)
				return 0, false
			}
			return got.Effective, true
		},
	})
	if failed != nil {
		return nil, failed
	}
	return &Proficiencies{Slots: f, Warnings: ws}, nil
}

// PreviewProficiencies is the pools as they would read after the edit (ADR 0037 step 12): each
// pool, each pick's figures and the warnings, refused as SetProficiencies refuses. It writes and
// logs nothing.
func (s *service) PreviewProficiencies(ctx context.Context, characterID int64, e Edit) (*Proficiencies, error) {
	var out *Proficiencies
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.View(ctx, characterID)
		if err != nil {
			return err
		}
		rs, err := s.rulesets.ForCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("character %d ruleset: %w", characterID, err)
		}
		next, langs, err := s.edited(c, e, rs.WeaponLift)
		if err != nil {
			return err
		}
		out, err = s.proficiencies(ctx, characterID, c, next, langs, rs.WeaponLift)
		return err
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
		c, err := s.chars.View(ctx, characterID)
		if err != nil {
			return err
		}
		rs, err := s.rulesets.ForCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("character %d ruleset: %w", characterID, err)
		}
		next, langs, err := s.edited(c, e, rs.WeaponLift)
		if err != nil {
			return err
		}
		before, beforeLangs := c.ProficiencyPicks.Clone(), c.Languages
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
		out, err = s.proficiencies(ctx, characterID, c, next, langs, rs.WeaponLift)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// restorePicks writes a snapshot's picks and languages back: undo the save's "before", redo its
// "after" (ADR 0019), inside the undo's transaction. A snapshot logged before a pick named a
// weapon (ADR 0037 step 21.7) carries no weapon codes, one logged before the languages moved to
// the Skills tab no languages: the stored ones stay (the weapon picks with their redo list). One
// logged before a class's free proficiencies cost 0 has their picks converted as the data step
// converted the stored ones (ADR 0037 step 22.7), on the character's class entries now.
func (s *service) restorePicks(ctx context.Context, characterID int64, _ string, st picksState) error {
	var stored charactersvc.View
	loaded := false
	load := func() error {
		if loaded {
			return nil
		}
		c, err := s.chars.View(ctx, characterID)
		stored, loaded = c, err == nil
		return err
	}
	picks := st.picks()
	if picks.Weapon == nil {
		if err := load(); err != nil {
			return err
		}
		kept := stored.ProficiencyPicks.Clone()
		picks.Weapon, picks.WeaponRedo = kept.Weapon, kept.WeaponRedo
	}
	if !st.FreeGrants {
		// ADR 0037 step 22.7: an entry logged before a free pick cost 0 never charges it again.
		if err := load(); err != nil {
			return err
		}
		picks.Nonweapon, _ = skills.ReturnFreeSlots(s.book, stored.SkillClasses(), picks.Nonweapon)
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
	if st.Languages != nil && !skills.LanguagesEqual(stored.Languages, *st.Languages) {
		if err := s.picks.SetLanguages(ctx, characterID, *st.Languages); err != nil {
			return fmt.Errorf("restore languages: %w", err)
		}
	}
	return nil
}

package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// PointsStateKey is the action-log state key of a thief's or a bard's skill
// points.
const PointsStateKey = "skill_points"

// Points writes the skill points of every skill set of a character (characters.skill_points,
// per set code). It joins the ctx transaction.
type Points interface {
	SetSkillPoints(ctx context.Context, characterID int64, points skills.SetPoints) error
	// SkillPoints are the class and the stored points of every skill set of a character: what an
	// undo needs to write one set back without deriving the sheet. domain.ErrCharacterNotFound
	// when it is missing.
	SkillPoints(ctx context.Context, characterID int64) (class string, points skills.SetPoints, err error)
}

// pointsSnaps is the snapshotter of the skill points (ADR 0018 step 8).
var pointsSnaps = actionlog.NewSnapshotter[pointsState](PointsStateKey)

// pointsState is the JSON encoding of a "skill_points" snapshot: the points per skill of one
// skill set, without zeros. An entry from before skill sets has no Set: it is the points of the
// character's only set. Entries already in the log must stay decodable: add fields, never
// rename.
type pointsState struct {
	Set    string         `json:"set,omitempty"`
	Points map[string]int `json:"points"`
}

// pointsRecord is the payload of a skill_points entry, the points of a skill set before and
// after the spend (no Set in an entry from before skill sets: its only set). The UI words it.
// Add fields, never rename.
type pointsRecord struct {
	Set    string         `json:"set,omitempty"`
	Before map[string]int `json:"before"`
	After  map[string]int `json:"after"`
}

// soleSet is the skill set a spend or a read addresses (ADR 0040 step 3.5): set when named,
// else the class's only set; a class with several needs the set named (a *domain.FieldError
// field set, code empty). "" for a class without a set: its points are refused as not allowed.
func soleSet(def classes.Class, set string) (string, error) {
	if set != "" {
		return set, nil
	}
	listed := def.ListsAll(classes.KindRogueSkills)
	switch len(listed) {
	case 0:
		return "", nil
	case 1:
		return listed[0].RogueSkills.Set, nil
	}
	return "", &domain.FieldError{Field: skills.FieldSet, Message: skills.CodeEmpty}
}

// classDef is class's definition (ADR 0040 step 5: the features the points read).
func (s *service) classDef(class string) (classes.Class, error) {
	def, ok := s.book.ClassDef(class)
	if !ok {
		return classes.Class{}, fmt.Errorf("%w %q", classes.ErrUnknownClass, class)
	}
	return def, nil
}

func orEmpty(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

// SetPoints is ADR 0030 steps 17 and 18 after creation: the player of a
// character with skill sets sets, for the skill set set ("": its only set,
// ADR 0040 step 3.5), the total points of each skill (those it leaves out are 0), checked as at
// creation by skills.ValidSkillPoints; at the realized level, a spend
// that leaves the budget over and spends more is refused (ADR 0037 step 10,
// skills.SkillPointsWithinBudget), an existing overspend that does
// not grow stays a warning of the derived sheet. It is one skill_points entry
// of the action log with the points' snapshot, so undo gives them back. The
// same points as stored record nothing. Returns the points as stored.
func (s *service) SetPoints(ctx context.Context, characterID int64, set string, points map[string]int) (map[string]int, error) {
	var out map[string]int
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		def, err := s.classDef(c.Class)
		if err != nil {
			return err
		}
		if set, err = soleSet(def, set); err != nil {
			return err
		}
		next, err := skills.ValidSkillPoints(def, set, points)
		if err != nil {
			return err
		}
		// ADR 0037 step 10: over the budget at the realized level and
		// spending more is refused before any write or log entry.
		if err := skills.SkillPointsWithinBudget(def, set, c.Derived.RealizedLevel, c.SkillPoints.Of(set), next); err != nil {
			return err
		}
		out = orEmpty(next)
		before := orEmpty(c.SkillPoints.Of(set))
		if maps.Equal(before, out) {
			return nil
		}
		if err := s.points.SetSkillPoints(ctx, characterID, c.SkillPoints.With(set, next)); err != nil {
			return fmt.Errorf("set skill points: %w", err)
		}
		payload, err := json.Marshal(pointsRecord{Set: set, Before: before, After: out})
		if err != nil {
			return fmt.Errorf("encode skill_points payload: %w", err)
		}
		b, a := pointsState{Set: set, Points: before}, pointsState{Set: set, Points: out}
		snap, err := pointsSnaps.Snapshot("", &b, &a)
		if err != nil {
			return err
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindSkillPoints, Payload: payload,
			Snapshots: []actionlog.Snapshot{snap},
		}); err != nil {
			return fmt.Errorf("record skill_points: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Module declares the codecs that write a thief's or a bard's skill points
// and the proficiency picks back: undo writes the save's "before", redo its
// "after" (ADR 0019). They run inside the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "skillsvc", Codecs: []actionlog.Codec{
		pointsSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st pointsState) error {
			return s.restorePoints(ctx, characterID, st)
		}, nil),
		picksSnaps.Restorer(s.restorePicks, nil),
	}}
}

// restorePoints writes the points of a snapshot's skill set back (undo writes the save's
// "before", redo its "after"), the other sets as they are stored; an entry from before skill sets
// restores the character's only set.
func (s *service) restorePoints(ctx context.Context, characterID int64, st pointsState) error {
	class, stored, err := s.points.SkillPoints(ctx, characterID)
	if err != nil {
		return fmt.Errorf("restore skill points: %w", err)
	}
	def, err := s.classDef(class)
	if err != nil {
		return err
	}
	set, err := soleSet(def, st.Set)
	if err != nil {
		return fmt.Errorf("restore skill points: %w", err)
	}
	if err := s.points.SetSkillPoints(ctx, characterID, stored.With(set, st.Points)); err != nil {
		return fmt.Errorf("restore skill points: %w", err)
	}
	return nil
}

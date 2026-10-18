package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// PointsStateKey is the action-log state key of a thief's or a bard's skill
// points.
const PointsStateKey = "skill_points"

// Points writes a thief's or a bard's skill points (characters.skill_points). It joins
// the ctx transaction.
type Points interface {
	SetSkillPoints(ctx context.Context, characterID int64, points map[string]int) error
}

// pointsState is the JSON encoding of a "skill_points" snapshot and the
// payload of a skill_points entry: the points per skill, without zeros.
// Entries already in the log must stay decodable: add fields, never rename.
type pointsState struct {
	Points map[string]int `json:"points"`
}

// pointsRecord is the payload of a skill_points entry, the points before
// and after the spend. The UI words it. Add fields, never rename.
type pointsRecord struct {
	Before map[string]int `json:"before"`
	After  map[string]int `json:"after"`
}

func orEmpty(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

// SetPoints is ADR 0030 steps 17 and 18 after creation: the player of a
// thief or a bard sets
// the total points of each skill (those it leaves out are 0), checked as at
// creation by charactersvc.ValidSkillPoints; the budget at the realized
// level stays a warning of the derived sheet. It is one skill_points entry
// of the action log with the points' snapshot, so undo gives them back. The
// same points as stored record nothing. Returns the points as stored.
func (s *service) SetPoints(ctx context.Context, characterID int64, points map[string]int) (map[string]int, error) {
	var out map[string]int
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		next, err := charactersvc.ValidSkillPoints(c.Class, points)
		if err != nil {
			return err
		}
		out = orEmpty(next)
		before := orEmpty(c.SkillPoints)
		if maps.Equal(before, out) {
			return nil
		}
		if err := s.points.SetSkillPoints(ctx, characterID, next); err != nil {
			return fmt.Errorf("set skill points: %w", err)
		}
		payload, err := json.Marshal(pointsRecord{Before: before, After: out})
		if err != nil {
			return fmt.Errorf("encode skill_points payload: %w", err)
		}
		b, err := json.Marshal(pointsState{Points: before})
		if err != nil {
			return fmt.Errorf("encode skill points snapshot: %w", err)
		}
		a, err := json.Marshal(pointsState{Points: out})
		if err != nil {
			return fmt.Errorf("encode skill points snapshot: %w", err)
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindSkillPoints, Payload: payload,
			Snapshots: []actionlog.Snapshot{{Key: PointsStateKey, Before: b, After: a}},
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

type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes a thief's or a bard's skill points back: undo writes the spend's
// "before", redo its "after" (ADR 0019). It runs inside the undo's
// transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		PointsStateKey: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var st pointsState
			if err := json.Unmarshal(state, &st); err != nil {
				return fmt.Errorf("decode skill points snapshot: %w", err)
			}
			if len(st.Points) == 0 {
				st.Points = nil
			}
			if err := s.points.SetSkillPoints(ctx, characterID, st.Points); err != nil {
				return fmt.Errorf("restore skill points: %w", err)
			}
			return nil
		}),
	}
}

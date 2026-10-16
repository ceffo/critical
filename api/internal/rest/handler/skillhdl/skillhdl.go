// Package skillhdl exposes the skill checks (ADR 0014): GET /api/skills
// (the proficiencies and bard skills with their targets and last checks),
// POST /api/roll/skill/{id} (one check, rolled and logged on the server, with
// an optional DM adjustment) and POST /api/roll/skill/{id}/adjust (the
// adjustment of the latest check, after the roll).
package skillhdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type skillSvc interface {
	List(ctx context.Context, characterID int64) (*skillsvc.Skills, error)
	Roll(ctx context.Context, characterID int64, id string, adj skillsvc.Adjustment) (*skillsvc.CheckResult, error)
	Adjust(ctx context.Context, characterID, entryID int64, adj skillsvc.Adjustment) (*skillsvc.CheckResult, error)
}

// RegisterRoutes wires the skill endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc skillsvc.SVC) {
	r.Get("/api/skills", render.Handle(List(charID, svc)))
	r.Post("/api/roll/skill/{id}", render.Handle(Roll(charID, svc)))
	r.Post("/api/roll/skill/{id}/adjust", render.Handle(Adjust(charID, svc)))
}

// AdjustmentDTO is the body of POST /api/roll/skill/{id} (optional, every
// field too: no body is no adjustment) and of POST
// /api/roll/skill/{id}/adjust (adjustment required, 0 removes it). The
// adjustment moves the target, + easier, - harder.
type AdjustmentDTO struct {
	Adjustment *int   `json:"adjustment"`
	Reason     string `json:"reason,omitempty"`
}

// LastDTO is the newest non-voided check of a skill: target is unadjusted,
// adjustment (with its reason, when set), adjustedTarget and success are the
// ones in effect.
type LastDTO struct {
	EntryID          int64  `json:"entryId"`
	Roll             int    `json:"roll"`
	Target           int    `json:"target"`
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	AdjustedTarget   int    `json:"adjustedTarget"`
	Success          bool   `json:"success"`
}

// ProficiencyDTO is one proficiency; target = abilityScore + modifier,
// computed by the server. last is null before the first check.
type ProficiencyDTO struct {
	ID           string   `json:"id"`
	Ability      string   `json:"ability"`
	AbilityScore int      `json:"abilityScore"`
	Modifier     int      `json:"modifier"`
	Target       int      `json:"target"`
	Last         *LastDTO `json:"last"`
}

// BardSkillDTO is one bard skill; last is null before the first check.
type BardSkillDTO struct {
	ID      string   `json:"id"`
	Percent int      `json:"percent"`
	Last    *LastDTO `json:"last"`
}

// SkillsDTO is the GET /api/skills answer.
type SkillsDTO struct {
	Proficiencies []ProficiencyDTO `json:"proficiencies"`
	BardSkills    []BardSkillDTO   `json:"bardSkills"`
}

// CheckDTO is one check: target is unadjusted, adjustedTarget = target +
// adjustment the one the roll is judged against, success the verdict in
// effect and rolledSuccess the verdict as rolled. ability, abilityScore and
// modifier are set for a proficiency only.
type CheckDTO struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Die              int    `json:"die"`
	Roll             int    `json:"roll"`
	Target           int    `json:"target"`
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	AdjustedTarget   int    `json:"adjustedTarget"`
	Success          bool   `json:"success"`
	RolledSuccess    bool   `json:"rolledSuccess"`
	Ability          string `json:"ability,omitempty"`
	AbilityScore     *int   `json:"abilityScore,omitempty"`
	Modifier         *int   `json:"modifier,omitempty"`
	LogEntryID       int64  `json:"logEntryId"`
}

func lastDTO(l *skillsvc.LastCheck) *LastDTO {
	if l == nil {
		return nil
	}
	return &LastDTO{
		EntryID: l.EntryID, Roll: l.Roll, Target: l.Target,
		Adjustment: l.Adjustment, AdjustmentReason: l.AdjustmentReason, AdjustedTarget: l.AdjustedTarget,
		Success: l.Success,
	}
}

// ToDTO maps the skills to their wire shape.
func ToDTO(s *skillsvc.Skills) SkillsDTO {
	out := SkillsDTO{
		Proficiencies: make([]ProficiencyDTO, 0, len(s.Proficiencies)),
		BardSkills:    make([]BardSkillDTO, 0, len(s.BardSkills)),
	}
	for _, p := range s.Proficiencies {
		out.Proficiencies = append(out.Proficiencies, ProficiencyDTO{
			ID: p.ID, Ability: string(p.Ability), AbilityScore: p.AbilityScore, Modifier: p.Modifier,
			Target: p.Target, Last: lastDTO(p.Last),
		})
	}
	for _, b := range s.BardSkills {
		out.BardSkills = append(out.BardSkills, BardSkillDTO{ID: b.ID, Percent: b.Percent, Last: lastDTO(b.Last)})
	}
	return out
}

// CheckToDTO maps a check to its wire shape.
func CheckToDTO(c *skillsvc.CheckResult) CheckDTO {
	out := CheckDTO{
		ID: c.ID, Kind: string(c.Kind), Die: c.Die, Roll: c.Roll, Target: c.Target,
		Adjustment: c.Adjustment, AdjustmentReason: c.AdjustmentReason, AdjustedTarget: c.AdjustedTarget,
		Success: c.Success, RolledSuccess: c.RolledSuccess, LogEntryID: c.LogEntryID,
	}
	if c.Kind == skillsvc.KindProficiency {
		score, mod := c.AbilityScore, c.Modifier
		out.Ability, out.AbilityScore, out.Modifier = string(c.Ability), &score, &mod
	}
	return out
}

// List returns every skill with its target and last check.
func List(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		s, err := svc.List(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(s))
		return nil
	}
}

// Roll checks one skill, its target moved by the optional adjustment; an
// unknown id is a 404, an adjustment out of range or a too long reason a 400.
func Roll(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req AdjustmentDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			return render.BadRequestError("invalid request body")
		}
		adj := skillsvc.Adjustment{Reason: req.Reason}
		if req.Adjustment != nil {
			adj.Value = *req.Adjustment
		}
		c, err := svc.Roll(r.Context(), charID(r), chi.URLParam(r, "id"), adj)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, CheckToDTO(c))
		return nil
	}
}

// Adjust sets the adjustment of the latest check (log entry {id}); the server
// judges the same roll against the moved target and returns the updated
// check. 409 when the entry is voided or not the latest roll, 400 when it is
// not a skill check or the adjustment is invalid, 404 when it does not exist.
func Adjust(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		var req AdjustmentDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.Adjustment == nil {
			return render.BadRequestError("adjustment is required")
		}
		c, err := svc.Adjust(r.Context(), charID(r), id, skillsvc.Adjustment{Value: *req.Adjustment, Reason: req.Reason})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, CheckToDTO(c))
		return nil
	}
}

// Package skillhdl exposes the skill checks (ADR 0014): GET /api/skills
// (the proficiencies and bard skills with their targets and last checks),
// POST /api/roll/skill/{id} (one check, rolled and logged on the server, with
// an optional DM adjustment) and POST /api/roll/skill/{id}/adjust (the
// adjustment of the latest check, after the roll), PUT /api/skills/points (the
// points of a skill set, ADR 0030 steps 17 and 18), PUT /api/proficiencies and POST
// /api/proficiencies/preview (the proficiency picks, ADR 0037 step 12); and the plain ability
// checks (ADR 0028): POST /api/roll/ability/{ability} and POST
// /api/roll/ability/{entryId}/adjust.
package skillhdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type skillSvc interface {
	List(ctx context.Context, characterID int64) (*skillsvc.Skills, error)
	RollInSet(ctx context.Context, characterID int64, set, id string, adj skillsvc.Adjustment) (*skillsvc.CheckResult, error)
	Adjust(ctx context.Context, characterID, entryID int64, adj skillsvc.Adjustment) (*skillsvc.CheckResult, error)
	RollAbility(ctx context.Context, characterID int64, ability string) (*skillsvc.AbilityCheckResult, error)
	AdjustAbility(ctx context.Context, characterID, entryID int64, adj skillsvc.Adjustment) (*skillsvc.AbilityCheckResult, error)
	SetPoints(ctx context.Context, characterID int64, set string, points map[string]int) (map[string]int, error)
	PreviewPoints(ctx context.Context, characterID int64, set string, points map[string]int) (*skillsvc.Skills, error)
}

// RegisterRoutes wires the skill endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc skillsvc.SVC) {
	r.Get("/api/skills", render.Handle(List(charID, svc)))
	r.Put("/api/skills/points", render.Handle(SetPoints(charID, svc)))
	r.Post("/api/skills/points/preview", render.Handle(PreviewPoints(charID, svc)))
	r.Put("/api/proficiencies", render.Handle(SetProficiencies(charID, svc)))
	r.Post("/api/proficiencies/preview", render.Handle(PreviewProficiencies(charID, svc)))
	r.Post("/api/roll/skill/{id}", render.Handle(Roll(charID, svc)))
	r.Post("/api/roll/skill/{id}/adjust", render.Handle(Adjust(charID, svc)))
	// {id} is the ability key on the roll, the log entry on the adjustment
	// (ADR 0028 steps 1 and 6), as on the skill routes.
	r.Post("/api/roll/ability/{id}", render.Handle(RollAbility(charID, svc)))
	r.Post("/api/roll/ability/{id}/adjust", render.Handle(AdjustAbility(charID, svc)))
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

// ProficiencyDTO is one picked proficiency with a check; target =
// abilityScore + modifier + extraSlots (the slots spent beyond the cost, ADR
// 0037 step 5), computed by the server. last is null before the first check.
type ProficiencyDTO struct {
	ID           string   `json:"id"`
	Ability      string   `json:"ability"`
	AbilityScore int      `json:"abilityScore"`
	Modifier     int      `json:"modifier"`
	ExtraSlots   int      `json:"extraSlots"`
	Target       int      `json:"target"`
	Last         *LastDTO `json:"last"`
}

// RogueSkillDTO is one thieving skill (ADR 0030 step 17) or bard skill
// (step 18): id the skill code, percent = base + points + race + dexterity +
// armor, capped at 95 and floored at 0; last is null before the first
// check. maxPercent is that cap and maxPoints the most points one skill
// takes (the editor's range).
type RogueSkillDTO struct {
	ID        string `json:"id"`
	Base      int    `json:"base"`
	Points    int    `json:"points"`
	Race      int    `json:"race"`
	Dexterity int    `json:"dexterity"`
	Armor     int    `json:"armor"`
	Percent   int    `json:"percent"`
	// RawPercent is the sum of the terms before the 95 cap and the 0 floor;
	// it differs from percent when one applied.
	RawPercent int `json:"rawPercent"`
	MaxPercent int `json:"maxPercent"`
	MaxPoints  int `json:"maxPoints"`
	// Usable is false while the skill cannot be rolled (under its set's least percentage, PHB p.39, or
	// one the worn armour blocks).
	Usable bool `json:"usable"`
	// Blocked: the worn body armour blocks the skill (ADR 0042 step 16.3, its roll refused with
	// skill_blocked_by_armour); absent when it does not.
	Blocked bool     `json:"blocked,omitempty"`
	Last    *LastDTO `json:"last"`
}

// SkillsDTO is the GET /api/skills answer.
type SkillsDTO struct {
	Proficiencies []ProficiencyDTO `json:"proficiencies"`
	// SkillSets is one set per rogue_skills feature the class grants (ADR 0040
	// step 3.5), empty for a class without skill sets.
	SkillSets []SkillSetDTO `json:"skillSets"`
}

// SkillSetDTO is one skill set of the tab: set the rogue_skills set code, name its heading (a
// language map, ADR 0040 step 15; its rolls are logged as rogueSkill with the set), skillArmor the skill
// armour it reads (ADR 0033 step 14, from the worn body armour: none, leather,
// padded_studded_leather, elven_chain or metal), budget its skill point budget with the terms of
// its formula (null before the level its points start) and skills its skills.
type SkillSetDTO struct {
	Set        string          `json:"set"`
	Name       localized.Text  `json:"name"`
	SkillArmor string          `json:"skillArmor"`
	Budget     *BudgetDTO      `json:"budget"`
	Skills     []RogueSkillDTO `json:"skills"`
}

// BudgetDTO is a thief's or a bard's skill point budget (wire.SkillPointsBudgetDTO, shared with
// the sheet's derived block).
type BudgetDTO = wire.SkillPointsBudgetDTO

// CheckDTO is one check: target is unadjusted, adjustedTarget = target +
// adjustment the one the roll is judged against, success the verdict in
// effect and rolledSuccess the verdict as rolled. ability, abilityScore and
// modifier are set for a proficiency only.
type CheckDTO struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Set is the skill set of a rogueSkill check; omitted for a proficiency.
	Set              string `json:"set,omitempty"`
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
	// ExtraSlots is a proficiency's slots beyond its cost (ADR 0037 step 5),
	// in target; set for a proficiency only.
	ExtraSlots *int  `json:"extraSlots,omitempty"`
	LogEntryID int64 `json:"logEntryId"`
	// Buff is the buff the verdict in effect put on (ADR 0037 step 20),
	// omitted when it put none; renewed is true when it replaced an active
	// one (step 20.3).
	Buff    *CheckBuffDTO `json:"buff,omitempty"`
	Renewed bool          `json:"renewed,omitempty"`
	// Warnings are the check's class warnings (ADR 0042 step 13:
	// dual_class_old_ability); absent when none.
	Warnings []string `json:"warnings,omitempty"`
}

// CheckBuffDTO is the buff a check put on: its row id, stored name, the
// proficiency it is named from, its effects (keyed like the buff API) and
// rounds, as the skill_check payload records it.
type CheckBuffDTO struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	ProficiencyID string         `json:"proficiencyId"`
	Effects       map[string]int `json:"effects"`
	Rounds        int            `json:"rounds"`
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
		SkillSets:     make([]SkillSetDTO, 0, len(s.SkillSets)),
	}
	for _, p := range s.Proficiencies {
		out.Proficiencies = append(out.Proficiencies, ProficiencyDTO{
			ID: p.ID, Ability: string(p.Ability), AbilityScore: p.AbilityScore, Modifier: p.Modifier, ExtraSlots: p.ExtraSlots,
			Target: p.Target, Last: lastDTO(p.Last),
		})
	}
	for _, set := range s.SkillSets {
		dto := SkillSetDTO{
			Set: set.Set, Name: set.Name, SkillArmor: set.Armor, Budget: wire.SkillPointsBudgetToDTO(set.Budget),
			Skills: make([]RogueSkillDTO, 0, len(set.Skills)),
		}
		for _, sk := range set.Skills {
			dto.Skills = append(dto.Skills, rogueSkillDTO(set.MinPercent, sk.SkillPercent, sk.Last))
		}
		out.SkillSets = append(out.SkillSets, dto)
	}
	return out
}

func rogueSkillDTO(minPercent int, p ruletablesvc.SkillPercent, last *skillsvc.LastCheck) RogueSkillDTO {
	return RogueSkillDTO{
		ID: p.Skill, Base: p.Base, Points: p.Points, Race: p.Race, Dexterity: p.Dexterity, Armor: p.Armor,
		Percent: p.Percent, RawPercent: p.Raw, MaxPercent: ruletablesvc.MaxSkillPercent, MaxPoints: charactersvc.MaxSkillPoints,
		Usable: skillsvc.SkillUsable(minPercent, p.Percent) && !p.Blocked, Blocked: p.Blocked, Last: lastDTO(last),
	}
}

// CheckToDTO maps a check to its wire shape.
func CheckToDTO(c *skillsvc.CheckResult) CheckDTO {
	out := CheckDTO{
		ID: c.ID, Kind: string(c.Kind), Set: c.Set, Die: c.Die, Roll: c.Roll, Target: c.Target,
		Adjustment: c.Adjustment, AdjustmentReason: c.AdjustmentReason, AdjustedTarget: c.AdjustedTarget,
		Success: c.Success, RolledSuccess: c.RolledSuccess, LogEntryID: c.LogEntryID, Warnings: c.Warnings,
	}
	if c.Kind == skillsvc.KindProficiency {
		score, mod, extra := c.AbilityScore, c.Modifier, c.ExtraSlots
		out.Ability, out.AbilityScore, out.Modifier, out.ExtraSlots = string(c.Ability), &score, &mod, &extra
	}
	if c.Buff != nil {
		b := c.Buff.Buff
		out.Buff = &CheckBuffDTO{ID: b.ID, Name: b.Name, ProficiencyID: b.ProficiencyID, Effects: skillsvc.EffectsByKey(b.Effects), Rounds: b.Duration}
		out.Renewed = c.Buff.Renewed
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

// PointsDTO is the body of PUT /api/skills/points and of its preview: the total points per skill
// code of the skill set set (a skill left out has 0). set may be left out for a class with one
// skill set; with several it is required (refused as `set` empty).
type PointsDTO struct {
	Set    string         `json:"set"`
	Points map[string]int `json:"points"`
}

// SetPoints sets a thief's skill points and answers the skills as GET
// /api/skills does. Points refused are a 400 naming the field
// (skillPoints.<skill>, or skillPoints for a class without thieving skills).
func SetPoints(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req PointsDTO
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || req.Points == nil {
			return render.BadRequestError("invalid request body")
		}
		if _, err := svc.SetPoints(r.Context(), charID(r), req.Set, req.Points); err != nil {
			return err
		}
		s, err := svc.List(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(s))
		return nil
	}
}

// PreviewPoints answers the skills as they would read with the points of the
// body, as PUT /api/skills/points would leave them, and writes nothing: the
// editor shows the percentages and the budget of the draft while the player
// types. Points refused are the same 400 as the PUT's.
func PreviewPoints(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req PointsDTO
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || req.Points == nil {
			return render.BadRequestError("invalid request body")
		}
		s, err := svc.PreviewPoints(r.Context(), charID(r), req.Set, req.Points)
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
		// ?set= names the skill set when two of the character's sets share the skill.
		c, err := svc.RollInSet(r.Context(), charID(r), r.URL.Query().Get("set"), chi.URLParam(r, "id"), adj)
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

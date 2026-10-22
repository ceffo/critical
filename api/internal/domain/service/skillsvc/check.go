package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// LastLookup is how many of the newest skill checks List reads to find the
// last check of each skill (the action log's maximum page).
const LastLookup = 500

// ChangeSkillAdjustment names a post-roll adjustment of a check in its
// correction step (actionlog.CorrectionStep.Field): from the adjustment in
// effect to the new one.
const ChangeSkillAdjustment = "skillAdjustment"

// checkRecord is skillsvc's JSON encoding of a check inside its skill_check
// entry (payload); an adjustment amend carries the recomputed record.
// Entries already in the log must stay decodable: add fields, never rename
// them (decodeCheck fills the ones older entries lack).
type checkRecord struct {
	ID     string `json:"id"`
	Kind   Kind   `json:"kind"`
	Die    int    `json:"die"`
	Roll   int    `json:"roll"`
	Target int    `json:"target"`
	// Adjustment moves Target to AdjustedTarget; Success is the verdict
	// against AdjustedTarget.
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	AdjustedTarget   int    `json:"adjustedTarget"`
	Success          bool   `json:"success"`
	// RolledSuccess is the verdict as rolled, with the adjustment set before
	// the roll, kept through any later adjustment. Nil only in entries logged
	// before adjustments existed, whose verdict never changed.
	RolledSuccess *bool `json:"rolledSuccess,omitempty"`
	// Set for a proficiency only.
	Ability      Ability `json:"ability,omitempty"`
	AbilityScore int     `json:"abilityScore,omitempty"`
	Modifier     int     `json:"modifier"`
	// ExtraSlots are the slots spent beyond the cost, each +1 to Target (ADR
	// 0037 step 5); absent when 0 (and in entries logged before).
	ExtraSlots int `json:"extraSlots,omitempty"`
	// PreviousSuccess and PreviousAdjustment are the verdict and adjustment
	// before the latest post-roll adjustment; set on adjustment amends only.
	PreviousSuccess    *bool `json:"previousSuccess,omitempty"`
	PreviousAdjustment *int  `json:"previousAdjustment,omitempty"`
	// Buff is the buff the verdict in effect put on (ADR 0037 step 20.4),
	// Renewed true when it replaced an active one (step 20.3). Absent when
	// it put none, and in entries logged before: they read as no buff.
	Buff    *buffRecord `json:"buff,omitempty"`
	Renewed bool        `json:"renewed,omitempty"`
	// Replaced are the rows the buff renewed (buffsvc.EncodeRows), given
	// back when an adjustment turns the success into a failure (step 20.8).
	Replaced json.RawMessage `json:"replaced,omitempty"`
}

// decodeCheck reads a check payload. An entry logged before adjustments has
// none: its adjusted target is its target and its verdict the rolled one.
func decodeCheck(raw []byte) (checkRecord, error) {
	var r checkRecord
	if err := json.Unmarshal(raw, &r); err != nil {
		return checkRecord{}, err
	}
	r.AdjustedTarget = AdjustedTarget(r.Target, r.Adjustment)
	if r.RolledSuccess == nil {
		rolled := r.Success
		r.RolledSuccess = &rolled
	}
	return r, nil
}

// settle moves the target by the adjustment and judges the stored roll
// against it (ADR 0014 steps 5 and 6): the roll never changes.
func (r *checkRecord) settle() {
	r.AdjustedTarget = AdjustedTarget(r.Target, r.Adjustment)
	r.Success = Success(r.Kind, r.Roll, r.AdjustedTarget)
}

func (r checkRecord) result(entryID int64) *CheckResult {
	rolled := r.Success
	if r.RolledSuccess != nil {
		rolled = *r.RolledSuccess
	}
	return &CheckResult{
		ID: r.ID, Kind: r.Kind, Die: r.Die, Roll: r.Roll, Target: r.Target,
		Adjustment: r.Adjustment, AdjustmentReason: r.AdjustmentReason, AdjustedTarget: r.AdjustedTarget,
		Success: r.Success, RolledSuccess: rolled,
		Ability: r.Ability, AbilityScore: r.AbilityScore, Modifier: r.Modifier, ExtraSlots: r.ExtraSlots, LogEntryID: entryID,
		Buff: r.Buff.applied(r.Renewed),
	}
}

// normalize trims the reason, refuses a too long one and drops it with a 0
// adjustment (nothing to explain). The range depends on the check's kind
// (checkRange).
func normalize(adj Adjustment) (Adjustment, error) {
	adj.Reason = strings.TrimSpace(adj.Reason)
	if utf8.RuneCountInString(adj.Reason) > MaxAdjustmentReasonLen {
		return Adjustment{}, fmt.Errorf("%w: reason must be at most %d characters", domain.ErrInvalidInput, MaxAdjustmentReasonLen)
	}
	if adj.Value == 0 {
		adj.Reason = ""
	}
	return adj, nil
}

// checkRange refuses an adjustment beyond the range of kind (ADR 0014 step
// 7).
func checkRange(k Kind, value int) error {
	limit := MaxAdjustment(k)
	if value < -limit || value > limit {
		return fmt.Errorf("%w: adjustment must be between -%d and +%d", domain.ErrInvalidInput, limit, limit)
	}
	return nil
}

// Roll rolls one check of skill id: a proficiency is one d20 against the
// ability score plus the modifier (ADR 0014 steps 1 and 2), a bard or a
// thieving skill one d100 against its derived percentage (step 3, ADR 0030
// steps 17 and 18), the target moved by the pre-roll
// adjustment (step 5). The check is a skill_check entry with no snapshots,
// recorded in the same transaction (step 4), unless its success puts a buff
// on (ADR 0037 step 20.4): the buff list's snapshot is then in the entry,
// so undoing the check takes the buff off. An unknown id or an invalid
// adjustment is refused before anything is rolled.
func (s *service) Roll(ctx context.Context, characterID int64, id string, adj Adjustment) (*CheckResult, error) {
	return s.RollInSet(ctx, characterID, "", id, adj)
}

// RollInSet is Roll for the skill id of the skill set set (ADR 0040 step 3.5: two sets can share
// a skill, each with its own percentage and kind); "" is the first set that holds the skill. A
// set the character has not, or a skill the set has not, wraps domain.ErrNotFound.
func (s *service) RollInSet(ctx context.Context, characterID int64, set, id string, adj Adjustment) (*CheckResult, error) {
	adj, err := normalize(adj)
	if err != nil {
		return nil, err
	}
	var res *CheckResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		rec, err := s.check(ctx, characterID, set, id, adj)
		if err != nil {
			return err
		}
		var snaps []actionlog.Snapshot
		if rec.Success { // ADR 0037 step 20.2: a failure puts nothing on
			snap, err := s.putBuff(ctx, characterID, &rec)
			if err != nil {
				return err
			}
			if snap != nil {
				snaps = append(snaps, *snap)
			}
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode skill check payload: %w", err)
		}
		entryID, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindSkillCheck,
			Payload:     payload,
			Snapshots:   snaps,
		})
		if err != nil {
			return fmt.Errorf("record skill check: %w", err)
		}
		res = rec.result(entryID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Adjust sets the adjustment of the latest check entryID after the roll
// (ADR 0014 step 6): it replaces the one in effect, 0 removes it and drops
// the reason, and the verdict is recomputed from the stored roll and target;
// the verdict as rolled is kept. The change is an amend correction, its own
// undoable step (step 7, ADR 0010 step 6); AmendLatest refuses an entry that
// is not a skill check, is voided or is not the latest roll. A verdict that
// changes puts the check's buff on or takes it off in the same correction
// (ADR 0037 step 20.8), which carries the buff list's snapshot, so undoing
// the correction restores the list.
func (s *service) Adjust(ctx context.Context, characterID, entryID int64, adj Adjustment) (*CheckResult, error) {
	adj, err := normalize(adj)
	if err != nil {
		return nil, err
	}
	var (
		rec checkRecord
		e   *actionlog.Entry
	)
	// One transaction for the amend and the buff writes: AmendLatest's
	// joins it, and fn writes with its ctx.
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		e, err = s.log.AmendLatest(ctx, characterID, entryID, actionlog.KindSkillCheck, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
			return s.amendCheck(ctx, characterID, e, adj, &rec)
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return rec.result(e.ID), nil
}

// amendCheck is Adjust's amendment of check e: the adjustment replaced, the
// verdict recomputed, the buff following it (ADR 0037 step 20.8); rec is the
// amended check.
func (s *service) amendCheck(ctx context.Context, characterID int64, e actionlog.Entry, adj Adjustment, out *checkRecord) (actionlog.PayloadAmendment, error) {
	rec, err := decodeCheck(e.EffectivePayload())
	if err != nil {
		return actionlog.PayloadAmendment{}, fmt.Errorf("decode skill check #%d: %w", e.ID, err)
	}
	if err := checkRange(rec.Kind, adj.Value); err != nil {
		return actionlog.PayloadAmendment{}, err
	}
	prevSuccess, prevAdj := rec.Success, rec.Adjustment
	rec.Adjustment, rec.AdjustmentReason = adj.Value, adj.Reason
	rec.settle()
	rec.PreviousSuccess, rec.PreviousAdjustment = &prevSuccess, &prevAdj
	snaps, err := s.followVerdict(ctx, characterID, e.Round, &rec, prevSuccess)
	if err != nil {
		return actionlog.PayloadAmendment{}, err
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return actionlog.PayloadAmendment{}, fmt.Errorf("encode skill check payload: %w", err)
	}
	*out = rec
	return actionlog.PayloadAmendment{Payload: payload, Snapshots: snaps, Change: actionlog.Change{
		Field: ChangeSkillAdjustment, From: prevAdj, To: rec.Adjustment,
	}}, nil
}

// check finds the skill, checks the adjustment's range for its kind and
// rolls its die.
func (s *service) check(ctx context.Context, characterID int64, set, id string, adj Adjustment) (checkRecord, error) {
	rec, err := s.find(ctx, characterID, set, id)
	if err != nil {
		return checkRecord{}, err
	}
	if err := checkRange(rec.Kind, adj.Value); err != nil {
		return checkRecord{}, err
	}
	// ADR 0030 step 17 (PHB p.39): a thieving skill under 1% cannot be used
	// until points raise it, whatever the DM's adjustment.
	if !SkillUsable(rec.Kind, rec.Target) {
		return checkRecord{}, &domain.FieldError{Field: "id", Message: CodeSkillUnusable}
	}
	rec.Roll = s.dice.RollDie(rec.Die)
	rec.Adjustment, rec.AdjustmentReason = adj.Value, adj.Reason
	rec.settle()
	rolled := rec.Success
	rec.RolledSuccess = &rolled
	return rec, nil
}

// checkable are the character's picks with a check (ADR 0037 steps 2 and 5):
// a catalogue proficiency with an ability, its target derived on the
// character. A pick with no check (Blind-fighting, an id the catalogue does
// not hold) is not rolled.
func checkable(char *charactersvc.Character) []skills.PickFigures {
	if char.Derived.Proficiencies == nil {
		return nil
	}
	var out []skills.PickFigures
	for _, p := range char.Derived.Proficiencies.Picks {
		if p.HasCheck {
			out = append(out, p)
		}
	}
	return out
}

// find is the unrolled check of skill id (of the skill set set, "" for the first that holds it): its kind, die and target.
func (s *service) find(ctx context.Context, characterID int64, set, id string) (checkRecord, error) {
	char, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return checkRecord{}, fmt.Errorf("get character: %w", err)
	}
	// ADR 0014 step 1 as amended by ADR 0037 step 5: the ability score plus
	// the catalogue's modifier plus the extra slots, derived on the
	// character.
	for _, p := range checkable(char) {
		if p.ID != id {
			continue
		}
		score, err := abilityScore(char, Ability(p.Ability))
		if err != nil {
			return checkRecord{}, fmt.Errorf("proficiency %q: %w", p.ID, err)
		}
		return checkRecord{
			ID: p.ID, Kind: KindProficiency, Die: ProficiencyDie, Target: p.Target,
			Ability: Ability(p.Ability), AbilityScore: score, Modifier: p.Modifier, ExtraSlots: p.Extra,
		}, nil
	}
	// ADR 0030 steps 17 and 18 (ADR 0014 step 3 as amended): a thieving or
	// a bard skill's target is the percentage the character service derives
	// (its terms are on the character).
	for _, ss := range char.Derived.SkillSets {
		if set != "" && ss.Set != set {
			continue
		}
		kind, ok := KindOfSet(ss.Set)
		if !ok {
			continue
		}
		for _, sk := range ss.Skills {
			if sk.Skill == id {
				return checkRecord{ID: sk.Skill, Kind: kind, Die: skillDie(kind), Target: sk.Percent}, nil
			}
		}
	}
	return checkRecord{}, fmt.Errorf("%w: unknown skill %q", domain.ErrNotFound, id)
}

// List returns the catalog with each proficiency's target and the last check
// of every skill (its adjustment and verdict in effect), read from the newest
// LastLookup non-voided checks.
func (s *service) List(ctx context.Context, characterID int64) (*Skills, error) {
	return s.skills(ctx, characterID, nil)
}

// PreviewPoints reads the skills with the points of skill set set replaced by points ("": the
// character's only set), refused as SetPoints refuses them (skills.ValidSkillPoints, then the
// budget, ADR 0037 step 10); the percentages and the budget are the skills module's figures of
// the draft (skills.SkillSetsOf), nothing is written.
func (s *service) PreviewPoints(ctx context.Context, characterID int64, set string, points map[string]int) (*Skills, error) {
	return s.skills(ctx, characterID, &draft{set: set, points: points})
}

// draft is the points of a skill set to read in place of the stored ones.
type draft struct {
	set    string
	points map[string]int
}

// withPoints is char derived with the draft points in place of its stored
// ones: refused as SetPoints refuses them, then the skills' figures of the
// draft at the realized level and the worn armour's skill armour.
func (s *service) withPoints(char *charactersvc.Character, d draft) error {
	def, err := s.classDef(char.Derived.FeatureClass(classes.KindRogueSkills, char.Class)) // ADR 0042 step 12
	if err != nil {
		return err
	}
	set, err := soleSet(def, d.set)
	if err != nil {
		return err
	}
	char.SkillPoints = char.SkillPoints.Resolve(def)
	next, err := skills.ValidSkillPoints(def, set, d.points)
	if err != nil {
		return err
	}
	if err := skills.SkillPointsWithinBudget(def, set, char.Derived.FeatureLevel(classes.KindRogueSkills), char.SkillPoints.Of(set), next); err != nil {
		return err
	}
	all := char.SkillPoints.With(set, next)
	if err := char.Derived.SetSkillFigures(s.tables, def, char, all); err != nil {
		return fmt.Errorf("preview skill points: %w", err)
	}
	char.SkillPoints = all
	return nil
}

// skills builds the tab; draft, when set, is the skill points to read in
// place of the stored ones.
func (s *service) skills(ctx context.Context, characterID int64, draft *draft) (*Skills, error) {
	var out *Skills
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		if draft != nil {
			if err := s.withPoints(char, *draft); err != nil {
				return err
			}
		}
		last, err := s.lastChecks(ctx, characterID)
		if err != nil {
			return err
		}
		profs := checkable(char)
		out = &Skills{Proficiencies: make([]Proficiency, 0, len(profs)), SkillSets: make([]SkillSet, 0, len(char.Derived.SkillSets))}
		for _, ss := range char.Derived.SkillSets {
			kind, _ := KindOfSet(ss.Set)
			set := SkillSet{Set: ss.Set, Kind: kind, Armor: ss.Armor, Budget: ss.Points, Skills: make([]SetSkill, 0, len(ss.Skills))}
			for _, sk := range ss.Skills {
				set.Skills = append(set.Skills, SetSkill{SkillPercent: sk, Last: last[lastKey(kind, sk.Skill)]})
			}
			out.SkillSets = append(out.SkillSets, set)
		}
		for _, p := range profs {
			score, err := abilityScore(char, Ability(p.Ability))
			if err != nil {
				return fmt.Errorf("proficiency %q: %w", p.ID, err)
			}
			out.Proficiencies = append(out.Proficiencies, Proficiency{
				ID: p.ID, Ability: Ability(p.Ability), AbilityScore: score, Modifier: p.Modifier, ExtraSlots: p.Extra,
				Target: p.Target, Last: last[lastKey(KindProficiency, p.ID)],
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// lastKey keys a skill's last check: two skill sets can hold the same skill, each with its own
// kind, so the id alone is not enough.
func lastKey(kind Kind, id string) string { return string(kind) + ":" + id }

// lastChecks maps each skill id to its newest non-voided check.
func (s *service) lastChecks(ctx context.Context, characterID int64) (map[string]*LastCheck, error) {
	entries, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindSkillCheck, LastLookup)
	if err != nil {
		return nil, fmt.Errorf("list skill checks: %w", err)
	}
	last := map[string]*LastCheck{}
	for _, e := range entries { // newest first
		rec, err := decodeCheck(e.EffectivePayload())
		if err != nil {
			return nil, fmt.Errorf("decode skill check #%d: %w", e.ID, err)
		}
		key := lastKey(rec.Kind, rec.ID)
		if _, seen := last[key]; seen || rec.ID == "" {
			continue
		}
		last[key] = &LastCheck{
			EntryID: e.ID, Roll: rec.Roll, Target: rec.Target,
			Adjustment: rec.Adjustment, AdjustmentReason: rec.AdjustmentReason, AdjustedTarget: rec.AdjustedTarget,
			Success: rec.Success,
		}
	}
	return last, nil
}

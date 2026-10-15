package skillsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// LastLookup is how many of the newest skill checks List reads to find the
// last check of each skill (the action log's maximum page).
const LastLookup = 500

// checkRecord is skillsvc's JSON encoding of a check inside its skill_check
// entry (payload). Entries already in the log must stay decodable: add
// fields, never rename them.
type checkRecord struct {
	ID      string `json:"id"`
	Kind    Kind   `json:"kind"`
	Die     int    `json:"die"`
	Roll    int    `json:"roll"`
	Target  int    `json:"target"`
	Success bool   `json:"success"`
	// Set for a proficiency only.
	Ability      Ability `json:"ability,omitempty"`
	AbilityScore int     `json:"abilityScore,omitempty"`
	Modifier     int     `json:"modifier"`
}

func (r checkRecord) result(entryID int64) *CheckResult {
	return &CheckResult{
		ID: r.ID, Kind: r.Kind, Die: r.Die, Roll: r.Roll, Target: r.Target, Success: r.Success,
		Ability: r.Ability, AbilityScore: r.AbilityScore, Modifier: r.Modifier, LogEntryID: entryID,
	}
}

// Roll rolls one check of skill id: a proficiency is one d20 against the
// ability score plus the modifier (ADR 0014 steps 1 and 2), a bard skill one
// d100 against its percentage (step 3). The check is a skill_check entry with
// no snapshots, recorded in the same transaction (step 4). An unknown id is
// refused before anything is rolled.
func (s *service) Roll(ctx context.Context, characterID int64, id string) (*CheckResult, error) {
	var res *CheckResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		rec, err := s.check(ctx, characterID, id)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("encode skill check payload: %w", err)
		}
		entryID, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindSkillCheck,
			Payload:     payload,
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

// check finds the skill and rolls its die.
func (s *service) check(ctx context.Context, characterID int64, id string) (checkRecord, error) {
	profs, err := s.dal.ListProficiencies(ctx)
	if err != nil {
		return checkRecord{}, fmt.Errorf("list proficiencies: %w", err)
	}
	for _, p := range profs {
		if p.ID != id {
			continue
		}
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return checkRecord{}, fmt.Errorf("get character: %w", err)
		}
		score, err := abilityScore(char, p.Ability)
		if err != nil {
			return checkRecord{}, fmt.Errorf("proficiency %q: %w", p.ID, err)
		}
		target := ProficiencyTarget(score, p.Modifier)
		roll := s.dice.RollDie(ProficiencyDie)
		return checkRecord{
			ID: p.ID, Kind: KindProficiency, Die: ProficiencyDie, Roll: roll, Target: target,
			Success: ProficiencySuccess(roll, target),
			Ability: p.Ability, AbilityScore: score, Modifier: p.Modifier,
		}, nil
	}
	skills, err := s.dal.ListBardSkills(ctx)
	if err != nil {
		return checkRecord{}, fmt.Errorf("list bard skills: %w", err)
	}
	for _, b := range skills {
		if b.ID != id {
			continue
		}
		roll := s.dice.RollDie(BardSkillDie)
		return checkRecord{
			ID: b.ID, Kind: KindBardSkill, Die: BardSkillDie, Roll: roll, Target: b.Percent,
			Success: BardSkillSuccess(roll, b.Percent),
		}, nil
	}
	return checkRecord{}, fmt.Errorf("%w: unknown skill %q", domain.ErrNotFound, id)
}

// List returns the catalog with each proficiency's target and the last check
// of every skill, read from the newest LastLookup non-voided checks.
func (s *service) List(ctx context.Context, characterID int64) (*Skills, error) {
	var out *Skills
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		profs, err := s.dal.ListProficiencies(ctx)
		if err != nil {
			return fmt.Errorf("list proficiencies: %w", err)
		}
		skills, err := s.dal.ListBardSkills(ctx)
		if err != nil {
			return fmt.Errorf("list bard skills: %w", err)
		}
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		last, err := s.lastChecks(ctx, characterID)
		if err != nil {
			return err
		}
		out = &Skills{Proficiencies: make([]Proficiency, 0, len(profs)), BardSkills: make([]BardSkill, 0, len(skills))}
		for _, p := range profs {
			score, err := abilityScore(char, p.Ability)
			if err != nil {
				return fmt.Errorf("proficiency %q: %w", p.ID, err)
			}
			out.Proficiencies = append(out.Proficiencies, Proficiency{
				ID: p.ID, Ability: p.Ability, AbilityScore: score, Modifier: p.Modifier,
				Target: ProficiencyTarget(score, p.Modifier), Last: last[p.ID],
			})
		}
		for _, b := range skills {
			out.BardSkills = append(out.BardSkills, BardSkill{ID: b.ID, Percent: b.Percent, Last: last[b.ID]})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// lastChecks maps each skill id to its newest non-voided check.
func (s *service) lastChecks(ctx context.Context, characterID int64) (map[string]*LastCheck, error) {
	entries, err := s.log.ActiveEntries(ctx, characterID, actionlog.KindSkillCheck, LastLookup)
	if err != nil {
		return nil, fmt.Errorf("list skill checks: %w", err)
	}
	last := map[string]*LastCheck{}
	for _, e := range entries { // newest first
		var rec checkRecord
		if err := json.Unmarshal(e.EffectivePayload(), &rec); err != nil {
			return nil, fmt.Errorf("decode skill check #%d: %w", e.ID, err)
		}
		if _, seen := last[rec.ID]; seen || rec.ID == "" {
			continue
		}
		last[rec.ID] = &LastCheck{EntryID: e.ID, Roll: rec.Roll, Target: rec.Target, Success: rec.Success}
	}
	return last, nil
}

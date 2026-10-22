package ruletablesadpt

import (
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// definitionsDTO is the format-11 artifact's classes, groups and races (ADR
// 0021 step 18, ADR 0040 steps 1, 2 and 7), as catalogueadpt passes them.
// The schema has already checked every field; only the ones the registry
// reads so far are mapped (the equipment and traits move with their readers,
// ADR 0040 step 11.3).
type definitionsDTO struct {
	Classes []classDefDTO `json:"classes"`
	Groups  []groupDefDTO `json:"groups"`
	Races   []raceDefDTO  `json:"races"`
}

type classDefDTO struct {
	ID              string               `json:"id"`
	Name            localized.Text       `json:"name"`
	Group           string               `json:"group"`
	PrimeRequisites []string             `json:"prime_requisites"`
	AbilityMinimums []abilityMinimumDTO  `json:"ability_minimums"`
	Alignments      []string             `json:"alignments"`
	Experience      []classExperienceDTO `json:"experience"`
	// SpellProgression names a slot table (ADR 0040 step 1.9), absent for none.
	SpellProgression string       `json:"spell_progression"`
	Features         []featureDTO `json:"features"`
}

// featureDTO is a class feature (ADR 0040 step 3); params is read by kind.
type featureDTO struct {
	Kind      string          `json:"kind"`
	FromLevel int             `json:"from_level"`
	Params    json.RawMessage `json:"params"`
}

// featureParamsDTO is the union of the kinds' params; the schema has checked
// that each kind carries only its own.
type featureParamsDTO struct {
	Table         string           `json:"table"`
	Ability       string           `json:"ability"`
	Major         []string         `json:"major"`
	Minor         []string         `json:"minor"`
	MinorMaxLevel int              `json:"minor_max_level"`
	Set           string           `json:"set"`
	Budget        *rogueBudgetDTO  `json:"budget"`
	Armour        []rogueArmourDTO `json:"armour"`
	MaxSpellLevel int              `json:"max_spell_level"`
	FreeSpell     *bool            `json:"free_spell_per_new_spell_level"`
}

type rogueBudgetDTO struct {
	First            int `json:"first"`
	PerLevel         int `json:"per_level"`
	PerSkillFirst    int `json:"per_skill_first"`
	PerSkillPerLevel int `json:"per_skill_per_level"`
}

type rogueArmourDTO struct {
	Code   string `json:"code"`
	Column string `json:"column"`
	Extra  int    `json:"extra"`
}

// feature maps a feature to the domain's, its params by kind (an unknown kind
// keeps no params and is refused by classes.New).
func (f featureDTO) feature() (classes.Feature, error) {
	out := classes.Feature{Kind: f.Kind, FromLevel: f.FromLevel}
	var p featureParamsDTO
	if len(f.Params) > 0 {
		if err := json.Unmarshal(f.Params, &p); err != nil {
			return classes.Feature{}, fmt.Errorf("feature %s params: %w", f.Kind, err)
		}
	}
	switch f.Kind {
	case classes.KindMeleeAttacksPerRound, classes.KindTurnUndead, classes.KindBackstab:
		out.Table = p.Table
	case classes.KindSpellFailure:
		out.Ability = p.Ability
	case classes.KindSphereAccess:
		out.Spheres = &classes.SphereAccess{Major: p.Major, Minor: p.Minor, MinorMaxLevel: p.MinorMaxLevel}
	case classes.KindRogueSkills:
		r := &classes.RogueSkills{Set: p.Set}
		if p.Budget != nil {
			r.Budget = classes.SkillBudget(*p.Budget)
		}
		for _, a := range p.Armour {
			r.Armour = append(r.Armour, classes.SkillArmour(a))
		}
		out.RogueSkills = r
	case classes.KindSpellbook:
		out.Spellbook = &classes.Spellbook{MaxSpellLevel: p.MaxSpellLevel, FreeSpellPerNewSpellLevel: p.FreeSpell != nil && *p.FreeSpell}
	}
	return out, nil
}

type abilityMinimumDTO struct {
	Ability string `json:"ability"`
	Min     int    `json:"min"`
}

type classExperienceDTO struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
}

type groupDefDTO struct {
	ID    string          `json:"id"`
	THAC0 []groupTHAC0DTO `json:"thac0"`
}

type groupTHAC0DTO struct {
	Level int `json:"level"`
	THAC0 int `json:"thac0"`
}

type raceDefDTO struct {
	ID           string               `json:"id"`
	Name         localized.Text       `json:"name"`
	Requirements []raceDefRequirement `json:"requirements"`
}

type raceDefRequirement struct {
	Ability string `json:"ability"`
	Min     int    `json:"min"`
	Max     int    `json:"max"`
}

// definitions maps the definitions to a validated class registry (the rules
// are classes.New's). The experience and THAC0 rows must be levels 1, 2, ...
// in order.
func definitions(raw []byte) (*classes.Registry, error) {
	var d definitionsDTO
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("definitions: %w", err)
	}
	cls := make([]classes.Class, len(d.Classes))
	for i, c := range d.Classes {
		xp := make([]int, len(c.Experience))
		for l, r := range c.Experience {
			if r.Level != l+1 {
				return nil, fmt.Errorf("class %q experience: row %d: level %d expected, got %d", c.ID, l, l+1, r.Level)
			}
			xp[l] = r.XP
		}
		mins := make([]classes.AbilityMinimum, len(c.AbilityMinimums))
		for j, m := range c.AbilityMinimums {
			mins[j] = classes.AbilityMinimum(m)
		}
		features := make([]classes.Feature, len(c.Features))
		for j, f := range c.Features {
			v, err := f.feature()
			if err != nil {
				return nil, fmt.Errorf("class %q: %w", c.ID, err)
			}
			features[j] = v
		}
		cls[i] = classes.Class{
			ID: c.ID, Name: c.Name, Group: c.Group, PrimeRequisites: c.PrimeRequisites,
			AbilityMinimums: mins, Alignments: c.Alignments, Experience: xp,
			SpellProgression: c.SpellProgression, Features: features,
		}
	}
	groups := make([]classes.Group, len(d.Groups))
	for i, g := range d.Groups {
		thac0 := make([]int, len(g.THAC0))
		for l, r := range g.THAC0 {
			if r.Level != l+1 {
				return nil, fmt.Errorf("group %q thac0: row %d: level %d expected, got %d", g.ID, l, l+1, r.Level)
			}
			thac0[l] = r.THAC0
		}
		groups[i] = classes.Group{ID: g.ID, THAC0: thac0}
	}
	races := make([]classes.Race, len(d.Races))
	for i, r := range d.Races {
		reqs := make([]classes.AbilityRange, len(r.Requirements))
		for j, a := range r.Requirements {
			reqs[j] = classes.AbilityRange(a)
		}
		races[i] = classes.Race{ID: r.ID, Name: r.Name, Requirements: reqs}
	}
	return classes.New(cls, groups, races)
}

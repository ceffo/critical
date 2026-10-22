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
// reads so far are mapped (the features, equipment and traits move with their
// readers, ADR 0040 step 11.3).
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
	ID string `json:"id"`
}

type raceDefDTO struct {
	ID   string         `json:"id"`
	Name localized.Text `json:"name"`
}

// definitions maps the definitions to a validated class registry (the rules
// are classes.New's). The experience rows must be levels 1, 2, ... in order.
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
		cls[i] = classes.Class{
			ID: c.ID, Name: c.Name, Group: c.Group, PrimeRequisites: c.PrimeRequisites,
			AbilityMinimums: mins, Alignments: c.Alignments, Experience: xp,
		}
	}
	groups := make([]string, len(d.Groups))
	for i, g := range d.Groups {
		groups[i] = g.ID
	}
	races := make([]classes.Race, len(d.Races))
	for i, r := range d.Races {
		races[i] = classes.Race(r)
	}
	return classes.New(cls, groups, races)
}

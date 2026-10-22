package composition_test

import (
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// tabSet is the skill set with code set of the skills tab, nil when the character has none.
func tabSet(s *skillsvc.Skills, set string) *skillsvc.SkillSet {
	for i := range s.SkillSets {
		if s.SkillSets[i].Set == set {
			return &s.SkillSets[i]
		}
	}
	return nil
}

// derivedSet is the skill set with code set of a derived block, the zero set when the character
// has none (so its budget warnings read as none).
func derivedSet(d charactersvc.Derived, set string) skills.SkillSet {
	for _, s := range d.SkillSets {
		if s.Set == set {
			return s
		}
	}
	return skills.SkillSet{}
}

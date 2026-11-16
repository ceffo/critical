// Package skilldescsvc serves the description of each skill of a skill set (ADR 0037 step 19.6,
// ADR 0021 step 33): the catalogue's texts, imported into localized_texts at start, as a language
// map. A set has its own text for a skill (the bard's Pick Pockets is not the thief's), so the key
// is the pair. The names stay i18n keys by id and the figures stay in the skill sets, so this holds
// nothing but the texts. It owns no state: the texts change only when a server imports a new
// artifact, so they are read once after the import, as the proficiency descriptions are.
package skilldescsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Description is the description of a skill of a skill set in every language the catalogue
	// holds (English and French, ADR 0021 step 33.1). domain.ErrNotFound for a pair the catalogue
	// does not hold.
	Description(ctx context.Context, set, skill string) (localized.Text, error)
}

type service struct {
	byPair map[catalogue.SkillPair]localized.Text
}

// New builds the service over the imported descriptions. A pair twice is an error: the import
// refuses it, so this can only be a wiring mistake.
func New(descriptions []catalogue.SkillDescription) (SVC, error) {
	byPair := make(map[catalogue.SkillPair]localized.Text, len(descriptions))
	for _, d := range descriptions {
		pair := catalogue.SkillPair{Set: d.Set, Skill: d.Skill}
		if _, dup := byPair[pair]; dup {
			return nil, fmt.Errorf("skill descriptions: %q of %q twice", d.Skill, d.Set)
		}
		byPair[pair] = d.Description
	}
	return &service{byPair: byPair}, nil
}

// Health always succeeds: the texts are in memory.
func (s *service) Health(context.Context) error { return nil }

// Close releases nothing.
func (s *service) Close(context.Context) error { return nil }

// Module declares no codec: the service writes nothing.
func (s *service) Module() domain.Module { return domain.Module{Name: "skilldescsvc"} }

// Description is ADR 0037 step 19.6. It returns a copy: the caller may keep it.
func (s *service) Description(_ context.Context, set, skill string) (localized.Text, error) {
	t, ok := s.byPair[catalogue.SkillPair{Set: set, Skill: skill}]
	if !ok {
		return nil, fmt.Errorf("skill %q of set %q: %w", skill, set, domain.ErrNotFound)
	}
	out := make(localized.Text, len(t))
	for lang, body := range t {
		out[lang] = body
	}
	return out, nil
}

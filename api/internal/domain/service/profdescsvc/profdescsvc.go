// Package profdescsvc serves the description of each non-weapon proficiency (ADR 0037 step 19.4,
// ADR 0021 step 16): the catalogue's texts, imported into localized_texts at start, as a language
// map. The names stay i18n keys and the figures stay in seeds/proficiencies.json, so this holds
// nothing but the texts. It owns no state: the texts change only when a server imports a new
// artifact, so they are read once after the import, as the spell catalogue is.
package profdescsvc

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
	// Description is the proficiency's description in every language the catalogue holds
	// (English and French, ADR 0021 step 16.1). domain.ErrNotFound for an id the catalogue does
	// not hold.
	Description(ctx context.Context, id string) (localized.Text, error)
}

type service struct {
	byID map[string]localized.Text
}

// New builds the service over the imported descriptions. An id twice is an error: the import
// refuses it, so this can only be a wiring mistake.
func New(descriptions []catalogue.ProficiencyDescription) (SVC, error) {
	byID := make(map[string]localized.Text, len(descriptions))
	for _, d := range descriptions {
		if _, dup := byID[d.ID]; dup {
			return nil, fmt.Errorf("proficiency descriptions: %q twice", d.ID)
		}
		byID[d.ID] = d.Description
	}
	return &service{byID: byID}, nil
}

// Health always succeeds: the texts are in memory.
func (s *service) Health(context.Context) error { return nil }

// Close releases nothing.
func (s *service) Close(context.Context) error { return nil }

// Module declares no codec: the service writes nothing.
func (s *service) Module() domain.Module { return domain.Module{Name: "profdescsvc"} }

// Description is ADR 0037 step 19.4. It returns a copy: the caller may keep it.
func (s *service) Description(_ context.Context, id string) (localized.Text, error) {
	t, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("proficiency %q: %w", id, domain.ErrNotFound)
	}
	out := make(localized.Text, len(t))
	for lang, body := range t {
		out[lang] = body
	}
	return out, nil
}

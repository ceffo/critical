package profdescsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/profdescsvc"
)

// ADR 0037 step 19.4: a description is served as a language map, a copy the caller may keep; an
// id the catalogue does not hold is domain.ErrNotFound.
func TestDescription_Step19_ServedAsALanguageMap(t *testing.T) {
	svc, err := profdescsvc.New([]catalogue.ProficiencyDescription{
		{ID: "tumbling", Description: localized.Text{"en": "Tumble.", "fr": "Cabriole."}},
	})
	require.NoError(t, err)
	got, err := svc.Description(context.Background(), "tumbling")
	require.NoError(t, err)
	assert.Equal(t, localized.Text{"en": "Tumble.", "fr": "Cabriole."}, got)
	got["en"] = "changed"
	again, _ := svc.Description(context.Background(), "tumbling")
	assert.Equal(t, "Tumble.", again["en"], "the caller holds a copy")

	_, err = svc.Description(context.Background(), "swordplay")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestNew_Step19_RefusesAnIdTwice(t *testing.T) {
	_, err := profdescsvc.New([]catalogue.ProficiencyDescription{{ID: "a"}, {ID: "a"}})
	assert.Error(t, err)
}

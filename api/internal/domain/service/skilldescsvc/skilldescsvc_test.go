package skilldescsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/skilldescsvc"
)

// ADR 0037 step 19.6: a description is served as a language map for the (set, skill) pair, a
// copy the caller may keep; the same skill in two sets has two texts; a pair the catalogue does
// not hold is domain.ErrNotFound.
func TestDescription_Step19_6_ServedPerSetAsALanguageMap(t *testing.T) {
	svc, err := skilldescsvc.New([]catalogue.SkillDescription{
		{Set: "thief", Skill: "pick_pockets", Description: localized.Text{"en": "Thief.", "fr": "Voleur."}},
		{Set: "bard", Skill: "pick_pockets", Description: localized.Text{"en": "Bard.", "fr": "Barde."}},
	})
	require.NoError(t, err)
	ctx := context.Background()

	got, err := svc.Description(ctx, "thief", "pick_pockets")
	require.NoError(t, err)
	assert.Equal(t, localized.Text{"en": "Thief.", "fr": "Voleur."}, got)
	got["en"] = "changed"
	again, _ := svc.Description(ctx, "thief", "pick_pockets")
	assert.Equal(t, "Thief.", again["en"], "the caller holds a copy")

	bard, err := svc.Description(ctx, "bard", "pick_pockets")
	require.NoError(t, err)
	assert.Equal(t, localized.Text{"en": "Bard.", "fr": "Barde."}, bard, "a set has its own text for a skill")

	for _, pair := range [][2]string{{"thief", "swordplay"}, {"mage", "pick_pockets"}, {"bard", "open_locks"}} {
		_, err = svc.Description(ctx, pair[0], pair[1])
		assert.ErrorIs(t, err, domain.ErrNotFound, "%v", pair)
	}
}

func TestNew_Step19_6_RefusesAPairTwice(t *testing.T) {
	_, err := skilldescsvc.New([]catalogue.SkillDescription{{Set: "a", Skill: "b"}, {Set: "a", Skill: "b"}})
	assert.Error(t, err)
}

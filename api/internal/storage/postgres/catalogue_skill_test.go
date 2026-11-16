package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// storedSkillDescriptions reads the fake's skill descriptions through the queries the server
// reads them with, keyed "set/skill".
func storedSkillDescriptions(t *testing.T, f *fakeStore) map[string]localized.Text {
	t.Helper()
	rows, err := f.SkillDescriptionTexts(context.Background())
	require.NoError(t, err)
	out := map[string]localized.Text{}
	for _, r := range rows {
		k := r.SkillSet + "/" + r.Skill
		if out[k] == nil {
			out[k] = localized.Text{}
		}
		out[k][r.Lang] = r.Body
	}
	return out
}

func wantSkillDescriptions(c catalogue.Catalogue) map[string]localized.Text {
	out := map[string]localized.Text{}
	for _, d := range c.SkillDescriptions {
		out[d.Set+"/"+d.Skill] = d.Description
	}
	return out
}

// ADR 0021 step 33.3: each skill description becomes localized_texts rows, English and French,
// keyed from the (set, skill) pair: the bard's Pick Pockets and the thief's are two rows.
func TestImportCatalogue_Step33_SkillDescriptionsInLocalizedTexts(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.Len(t, cat.SkillDescriptions, 12)
	require.NoError(t, importOn(f, cat, nil))

	got := storedSkillDescriptions(t, f)
	assert.Equal(t, wantSkillDescriptions(cat), got)
	assert.Equal(t, []string{"en", "fr"}, got["thief/pick_pockets"].Languages())
	assert.NotEqual(t, got["thief/pick_pockets"], got["bard/pick_pockets"], "a set has its own text for a skill")
	assertTextIntegrity(t, f)

	// The server reads them back as language maps (ADR 0037 step 19.6).
	d := &DB{q: f}
	read, err := d.SkillDescriptions(context.Background())
	require.NoError(t, err)
	assert.Len(t, read, 12)
	assert.Equal(t, "bard", read[0].Set, "the rows come in the order of the pairs")
}

// ADR 0021 step 33.3: a second import of the same artifact changes nothing; a new version
// rewrites only the texts that changed, keeps a row's text key, and deletes a row the artifact no
// longer lists with its texts.
func TestImportCatalogue_Step33_ReimportChangesNothing(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	before := copyTables(f.t)

	require.NoError(t, importOn(f, cat, nil))
	assert.Equal(t, before, f.t, "the same format and version writes nothing")

	next := withVersion(cat, "1970-01-02", nil)
	next.SkillDescriptions = append([]catalogue.SkillDescription(nil), cat.SkillDescriptions...)
	first := cat.SkillDescriptions[0]
	next.SkillDescriptions[0] = catalogue.SkillDescription{Set: first.Set, Skill: first.Skill, Description: localized.Text{"en": "Changed.", "fr": "Modifié."}}
	drop := cat.SkillDescriptions[1]
	droppedKey := f.t.SkillDescs[drop.Set+"/"+drop.Skill]
	next.SkillDescriptions = append(next.SkillDescriptions[:1:1], next.SkillDescriptions[2:]...)
	key := f.t.SkillDescs[first.Set+"/"+first.Skill]
	require.NoError(t, importOn(f, next, nil))

	assert.Equal(t, wantSkillDescriptions(next), storedSkillDescriptions(t, f))
	assert.Equal(t, key, f.t.SkillDescs[first.Set+"/"+first.Skill], "a row keeps its text key")
	assert.NotContains(t, f.t.SkillDescs, drop.Set+"/"+drop.Skill)
	assert.NotContains(t, f.t.Texts, droppedKey, "the removed row's texts go with it")
	assertTextIntegrity(t, f)
}

// ADR 0021 step 33.3 (and step 5.5): a failed import changes nothing, the skill descriptions
// included.
func TestImportCatalogue_Step33_FailedImportChangesNothing(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	before := copyTables(f.t)
	err := importCatalogue(context.Background(), f, failingStore{f}, cat, nil)
	require.ErrorContains(t, err, "boom")
	assert.Equal(t, before, f.t, "no spell, no description, no text is left")
	assert.Empty(t, storedSkillDescriptions(t, f))
}

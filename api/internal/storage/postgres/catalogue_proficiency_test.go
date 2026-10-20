package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// storedDescriptions reads the fake's proficiency descriptions through the queries the server
// reads them with.
func storedDescriptions(t *testing.T, f *fakeStore) map[string]localized.Text {
	t.Helper()
	rows, err := f.ProficiencyDescriptionTexts(context.Background())
	require.NoError(t, err)
	out := map[string]localized.Text{}
	for _, r := range rows {
		if out[r.ProficiencyID] == nil {
			out[r.ProficiencyID] = localized.Text{}
		}
		out[r.ProficiencyID][r.Lang] = r.Body
	}
	return out
}

func wantDescriptions(c catalogue.Catalogue) map[string]localized.Text {
	out := map[string]localized.Text{}
	for _, d := range c.ProficiencyDescriptions {
		out[d.ID] = d.Description
	}
	return out
}

// ADR 0021 step 16.3: each description becomes localized_texts rows, English and French, keyed
// from the proficiency id.
func TestImportCatalogue_Step16_DescriptionsInLocalizedTexts(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.Len(t, cat.ProficiencyDescriptions, 65)
	require.NoError(t, importOn(f, cat, nil))

	got := storedDescriptions(t, f)
	assert.Equal(t, wantDescriptions(cat), got)
	assert.Equal(t, []string{"en", "fr"}, got["agriculture"].Languages())
	assertTextIntegrity(t, f)

	// The server reads them back as language maps (ADR 0037 step 19.4).
	d := &DB{q: f}
	read, err := d.ProficiencyDescriptions(context.Background())
	require.NoError(t, err)
	assert.Len(t, read, 65)
	assert.Equal(t, cat.ProficiencyDescriptions[0].ID, read[0].ID)
}

// ADR 0021 step 16.3: a second import of the same artifact changes nothing; a new version
// rewrites only the texts that changed, keeps a row's text key, and deletes a row the artifact no
// longer lists with its texts.
func TestImportCatalogue_Step16_ReimportChangesNothing(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	before := copyTables(f.t)

	require.NoError(t, importOn(f, cat, nil))
	assert.Equal(t, before, f.t, "the same format and version writes nothing")

	next := withVersion(cat, "1970-01-02", nil)
	next.ProficiencyDescriptions = append([]catalogue.ProficiencyDescription(nil), cat.ProficiencyDescriptions...)
	next.ProficiencyDescriptions[0] = catalogue.ProficiencyDescription{
		ID: cat.ProficiencyDescriptions[0].ID, Description: localized.Text{"en": "Changed.", "fr": "Modifié."},
	}
	dropped := next.ProficiencyDescriptions[1].ID
	droppedKey := f.t.ProfDescs[dropped]
	next.ProficiencyDescriptions = append(next.ProficiencyDescriptions[:1:1], next.ProficiencyDescriptions[2:]...)
	key := f.t.ProfDescs[next.ProficiencyDescriptions[0].ID]
	require.NoError(t, importOn(f, next, nil))

	assert.Equal(t, wantDescriptions(next), storedDescriptions(t, f))
	assert.Equal(t, key, f.t.ProfDescs[next.ProficiencyDescriptions[0].ID], "a row keeps its text key")
	assert.NotContains(t, f.t.ProfDescs, dropped)
	assert.NotContains(t, f.t.Texts, droppedKey, "the removed row's texts go with it")
	assertTextIntegrity(t, f)
}

// failingStore fails the version record, the last write of the import's transaction.
type failingStore struct{ *fakeStore }

func (s failingStore) RecordCatalogueVersion(context.Context, storeq.RecordCatalogueVersionParams) error {
	return errors.New("boom")
}

// ADR 0021 step 16.3 (and step 5.5): a failed import changes nothing, the descriptions included.
func TestImportCatalogue_Step16_FailedImportChangesNothing(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	before := copyTables(f.t)
	err := importCatalogue(context.Background(), f, failingStore{f}, cat, nil)
	require.ErrorContains(t, err, "boom")
	assert.Equal(t, before, f.t, "no spell, no description, no text is left")
	assert.Empty(t, storedDescriptions(t, f))
}

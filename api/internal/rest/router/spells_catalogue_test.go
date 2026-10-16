package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// spellsJSON returns the items of GET /api/spells?<query> by id, as raw JSON
// objects (so an absent field is visible).
func spellsJSON(t *testing.T, srv *httptest.Server, query string) map[string]map[string]any {
	t.Helper()
	res, err := http.Get(srv.URL + "/api/spells?" + query) //nolint:noctx // test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	out := map[string]map[string]any{}
	for _, it := range page.Items {
		out[it["id"].(string)] = it
	}
	return out
}

// Every text of a spell payload is a language map (ADR 0023 step 6) holding
// exactly the languages the catalogue has: French and English for Magic
// Missile's name and description in the fixture, English only for Armor's
// (no "fr" key: the client falls back to English) and for the stat texts.
func TestSpells_TextsAreLanguageMapsFromTheCatalogue(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:       filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir:  filepath.Join("..", "..", "..", "testdata", "seeds"),
		Catalogue: filepath.Join("..", "..", "..", "testdata", "catalogue.json"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	srv := httptest.NewServer(router.New(slog.New(slog.NewTextHandler(io.Discard, nil)), bs))
	t.Cleanup(srv.Close)

	mm := spellsJSON(t, srv, "q=Magic+Missile")["magic_missile"]
	require.NotNil(t, mm)
	assert.Equal(t, map[string]any{"en": "Magic Missile", "fr": "Projectile magique"}, mm["name"])
	desc, ok := mm["description"].(map[string]any)
	require.True(t, ok, "description is a language map")
	assert.Contains(t, desc["en"], "Fixture text for Magic Missile")
	assert.Contains(t, desc["fr"], "Texte de test pour Projectile magique")
	assert.Equal(t, map[string]any{"en": "60 yds. + 10 yds./level"}, mm["range"])
	assert.Equal(t, "PHB", mm["source"])

	armor := spellsJSON(t, srv, "q=Armor")["armor"]
	require.NotNil(t, armor)
	assert.Equal(t, map[string]any{"en": "Armor"}, armor["name"], "no French name: no fr key")
	adesc, ok := armor["description"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, adesc["en"])
	assert.NotContains(t, adesc, "fr")
	assert.Equal(t, map[string]any{}, armor["sphere"], "no sphere: an empty map")

	// French search through the real stack (ADR 0023 step 7).
	assert.Contains(t, spellsJSON(t, srv, "q=projectile"), "magic_missile")

	res, err := http.Get(srv.URL + "/api/spells/names")
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var names []map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&names))
	byID := map[string]any{}
	for _, n := range names {
		assert.Len(t, n, 2, "id and name only")
		byID[n["id"].(string)] = n["name"]
	}
	assert.Equal(t, map[string]any{"en": "Magic Missile", "fr": "Projectile magique"}, byID["magic_missile"])
	assert.Equal(t, map[string]any{"en": "Armor"}, byID["armor"])
}

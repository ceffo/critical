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

// The spell payloads carry the catalogue's French name and description when
// it has them (Magic Missile in the fixture), and omit both when it has none
// (Armor), so the client falls back to the English name and description.
func TestSpells_FrenchTextsFromTheCatalogueWithEnglishFallback(t *testing.T) {
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
	assert.Equal(t, "Magic Missile", mm["name"])
	assert.Equal(t, "Projectile magique", mm["name_fr"])
	assert.Contains(t, mm["description"], "Fixture text for Magic Missile")
	assert.Contains(t, mm["description_fr"], "Texte de test pour Projectile magique")
	assert.Equal(t, "PHB", mm["source"])

	armor := spellsJSON(t, srv, "q=Armor")["armor"]
	require.NotNil(t, armor)
	assert.Equal(t, "Armor", armor["name"])
	assert.NotEmpty(t, armor["description"])
	assert.NotContains(t, armor, "name_fr", "no French name: absent, the client shows the English one")
	assert.NotContains(t, armor, "description_fr")
}

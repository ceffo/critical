package router_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/powersvc"
	"github.com/ceffo/critical/api/internal/rest/handler/powerhdl"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// powerPage reads the ids and the total of a page; the costs are read as raw JSON.
type powerPage struct {
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
	Total int `json:"total"`
}

func powersRouter(t *testing.T) (http.Handler, catalogueadpt.Artifact) {
	t.Helper()
	art, err := catalogueadpt.LoadArtifact("../../../testdata/catalogue.json")
	require.NoError(t, err)
	svc, err := powersvc.New(art.Disciplines, art.Powers)
	require.NoError(t, err)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return router.New(log, &composition.BackingServices{Auth: signedIn(t), Powers: svc}, router.Options{}), art
}

// ADR 0043 step 8: the catalogue's powers are all served (the count matches the
// artifact), searchable by the English or the French name, filtered by
// discipline and kind, with every text a language map and a cost that is a
// number or a word; an unknown id is a 404 and a bad filter a 400.
func TestPowers_Step8_ServedFromTheCatalogueAsLanguageMaps(t *testing.T) {
	h, art := powersRouter(t)

	rec := do(h, "GET", "/api/powers", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var page powerPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	assert.Equal(t, len(art.Powers), page.Total, "every artifact power is served")
	assert.Len(t, page.Items, len(art.Powers))

	for _, tc := range []struct{ q, id string }{{"aura sight", "aura_sight"}, {"VISION de l'aura", "aura_sight"}, {"telekinesie", "telekinesis"}} {
		rec = do(h, "GET", "/api/powers?q="+url.QueryEscape(tc.q), "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		page = powerPage{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		require.NotEmpty(t, page.Items, tc.q)
		assert.Equal(t, tc.id, page.Items[0].ID, tc.q)
	}

	rec = do(h, "GET", "/api/powers?discipline=telepathy&kind=attack_mode", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	page = powerPage{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Items, 5)
	assert.Equal(t, "psionic_blast", page.Items[0].ID, "book order")
	assert.Equal(t, "mind_thrust", page.Items[1].ID)

	rec = do(h, "GET", "/api/powers/telekinesis", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.Equal(t, "variable", raw["initial_cost"], "a word cost is a string")
	assert.Equal(t, map[string]any{"en": "Telekinesis", "fr": "Télékinésie"}, raw["name"])
	assert.Contains(t, raw["description"], "fr")
	rec = do(h, "GET", "/api/powers/aura_sight", "")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.EqualValues(t, 9, raw["initial_cost"], "a numeric cost is a number")

	rec = do(h, "GET", "/api/powers/filters", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var opts powerhdl.FilterOptionsDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &opts))
	assert.Len(t, opts.Disciplines, 6)
	assert.Equal(t, []string{"science", "devotion", "defence_mode", "attack_mode"}, opts.Kinds)

	rec = do(h, "GET", "/api/powers/levitation", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"not_found"`)
	assert.Equal(t, http.StatusBadRequest, do(h, "GET", "/api/powers?kind=spell", "").Code)
	assert.Equal(t, http.StatusBadRequest, do(h, "GET", "/api/powers?discipline=pyromancy", "").Code)
	assert.Equal(t, http.StatusBadRequest, do(h, "GET", "/api/powers?page_size=900", "").Code)
}

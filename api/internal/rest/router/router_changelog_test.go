package router_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/changelog"
	"github.com/ceffo/critical/api/internal/rest/router"
)

func changelogRouter(t *testing.T, n int) http.Handler {
	t.Helper()
	vs := make([]changelog.Version, n)
	for i := range vs {
		vs[i] = changelog.Version{Version: fmt.Sprintf("0.%d.0", n-i), Features: []string{"Feature " + fmt.Sprint(n-i)}, Fixes: []string{}}
	}
	log, err := changelog.New(vs)
	require.NoError(t, err)
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	return router.New(l, &composition.BackingServices{Auth: signedIn(t)}, router.Options{Changelog: log})
}

type changelogPage struct {
	Versions []struct {
		Version  string   `json:"version"`
		Features []string `json:"features"`
		Fixes    []string `json:"fixes"`
	} `json:"versions"`
	NextCursor string `json:"nextCursor"`
}

func getChangelog(t *testing.T, h http.Handler, query string) (*httptest.ResponseRecorder, changelogPage) {
	t.Helper()
	rec := do(h, "GET", "/api/changelog"+query, "")
	var p changelogPage
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	}
	return rec, p
}

// ADR 0039 step 6: GET /api/changelog pages through the build's versions newest first, by cursor.
func TestChangelogEndpoint_Step6_PaginatesNewestFirst(t *testing.T) {
	h := changelogRouter(t, 5)

	rec, p := getChangelog(t, h, "?limit=2")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, p.Versions, 2)
	assert.Equal(t, "0.5.0", p.Versions[0].Version)
	assert.Equal(t, []string{"Feature 5"}, p.Versions[0].Features)
	assert.Equal(t, "0.4.0", p.NextCursor)

	_, p = getChangelog(t, h, "?limit=2&cursor="+p.NextCursor)
	require.Len(t, p.Versions, 2)
	assert.Equal(t, "0.3.0", p.Versions[0].Version)

	rec, p = getChangelog(t, h, "?limit=2&cursor=0.2.0")
	require.Len(t, p.Versions, 1)
	assert.Equal(t, "0.1.0", p.Versions[0].Version)
	assert.Empty(t, p.NextCursor)
	assert.NotContains(t, rec.Body.String(), "nextCursor", "the last page names no cursor")
	assert.Contains(t, rec.Body.String(), `"fixes":[]`, "an empty list is [] and not null")
}

// A build without changelog (a dev build) answers an empty list, not an error.
func TestChangelogEndpoint_Step6_ADevBuildIsEmpty(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router.New(l, &composition.BackingServices{Auth: signedIn(t)}, router.Options{})
	rec, p := getChangelog(t, h, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"versions":[]}`, rec.Body.String())
	assert.Empty(t, p.Versions)
}

func TestChangelogEndpoint_Step6_LimitDefaultsAndIsCapped(t *testing.T) {
	h := changelogRouter(t, changelog.MaxLimit+5)
	_, p := getChangelog(t, h, "")
	assert.Len(t, p.Versions, changelog.DefaultLimit)
	_, p = getChangelog(t, h, "?limit=100000")
	assert.Len(t, p.Versions, changelog.MaxLimit)
}

func TestChangelogEndpoint_Step6_RefusesABadLimitOrCursor(t *testing.T) {
	h := changelogRouter(t, 3)
	for _, q := range []string{"?limit=0", "?limit=-1", "?limit=abc", "?cursor=9.9.9"} {
		rec := do(h, "GET", "/api/changelog"+q, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
	}
}

package router_test

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/rest/handler/webhdl"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// webBuild writes a minimal web build (index.html, a hashed asset, a public
// file) and returns its directory.
func webBuild(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":           "<!doctype html><title>critical</title>",
		"assets/index-a1b2.js": "console.log(1)",
		"favicon.svg":          "<svg/>",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	return dir
}

func webRouter(t *testing.T, web http.Handler) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return router.New(log, &composition.BackingServices{CharacterID: 1, Auth: signedIn(t)}, router.Options{Web: web, Commit: "abc123", Version: "1.2.3"})
}

const notFoundBody = `{"code":"not_found","error":"not found: /api/nope"}`

// Step 1.1: with a web build, a non-/api path is its file, or index.html when
// no file has that path (SPA fallback); unknown /api paths keep the JSON 404.
func TestWeb_Step1_1_ServesTheBuildWithSPAFallback(t *testing.T) {
	web, err := webhdl.New(webBuild(t))
	require.NoError(t, err)
	h := webRouter(t, web)

	for _, p := range []string{"/", "/index.html", "/spells/42", "/character/details?tab=1", "/assets/missing.js"} {
		rec := do(h, "GET", p, "")
		require.Equal(t, http.StatusOK, rec.Code, p)
		assert.Contains(t, rec.Body.String(), "<title>critical</title>", p)
		assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"), p)
	}

	asset := do(h, "GET", "/assets/index-a1b2.js", "")
	require.Equal(t, http.StatusOK, asset.Code)
	assert.Equal(t, "console.log(1)", asset.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", asset.Header().Get("Cache-Control"))

	icon := do(h, "GET", "/favicon.svg", "")
	require.Equal(t, http.StatusOK, icon.Code)
	assert.Equal(t, "<svg/>", icon.Body.String())
	assert.Equal(t, "no-cache", icon.Header().Get("Cache-Control"))

	head := do(h, "HEAD", "/spells", "")
	assert.Equal(t, http.StatusOK, head.Code)

	for _, p := range []string{"/api/nope", "/api", "/api/"} {
		rec := do(h, "GET", p, "")
		require.Equal(t, http.StatusNotFound, rec.Code, p)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), p)
	}
	assert.JSONEq(t, notFoundBody, do(h, "GET", "/api/nope", "").Body.String())

	assert.Equal(t, http.StatusMethodNotAllowed, do(h, "POST", "/spells", "").Code)
}

// Step 1.1: a path that climbs out of the web dir never reads outside it.
func TestWeb_Step1_1_StaysInsideTheWebDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "web")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "secret.json"), []byte("secret"), 0o600))
	web, err := webhdl.New(dir)
	require.NoError(t, err)
	h := webRouter(t, web)

	for _, p := range []string{"/../secret.json", "/a/../../secret.json", "/%2e%2e/secret.json"} {
		rec := do(h, "GET", p, "")
		assert.NotContains(t, rec.Body.String(), "secret", p)
	}
}

// Step 1.1: a web dir without index.html is refused when the server starts.
func TestWeb_Step1_1_RefusesADirWithoutIndex(t *testing.T) {
	_, err := webhdl.New(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index.html")
}

// Step 1.2: without a web dir (as in `just dev`), nothing is served outside
// /api: every unrouted path is the JSON 404, the API is unchanged.
func TestWeb_Step1_2_NoWebDirServesNothingOutsideAPI(t *testing.T) {
	h := webRouter(t, nil)
	for _, p := range []string{"/", "/index.html", "/spells/42", "/api/nope"} {
		rec := do(h, "GET", p, "")
		require.Equal(t, http.StatusNotFound, rec.Code, p)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), p)
	}
	assert.JSONEq(t, notFoundBody, do(h, "GET", "/api/nope", "").Body.String())
	assert.Equal(t, http.StatusOK, do(h, "GET", "/api/health", "").Code)
}

// Step 1.4: /api/health reports the commit the binary was built from.
func TestHealth_Step1_4_ReportsTheBuildCommit(t *testing.T) {
	rec := do(webRouter(t, nil), "GET", "/api/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok","commit":"abc123","version":"1.2.3"}`, rec.Body.String())
}

// Step 1.4: /api/health reports the release version baked into the image next
// to the commit.
func TestHealth_Step1_4_ReportsTheBuildVersion(t *testing.T) {
	rec := do(webRouter(t, nil), "GET", "/api/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"version":"1.2.3"`)
}

// Step 1.4: a build without a version or a commit (as in `just dev`) reports
// them as unknown.
func TestHealth_Step1_4_UnknownWithoutABuildIdentity(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router.New(log, &composition.BackingServices{CharacterID: 1}, router.Options{})
	rec := do(h, "GET", "/api/health", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok","commit":"unknown","version":"unknown"}`, rec.Body.String())
}

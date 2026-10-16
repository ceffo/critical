// Package webhdl serves the built web app (ADR 0024 step 1.1): every non-/api
// path is a file of the web build, or index.html when no such file exists (SPA
// fallback), so a deep link reloads into the app.
package webhdl

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// indexFile is the web build's entry page and the SPA fallback.
const indexFile = "index.html"

// assetsDir holds Vite's content-hashed bundles: a changed file gets a new
// name, so they may be cached forever.
const assetsDir = "assets/"

const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// Handler serves the files of a web build directory.
type Handler struct {
	fsys fs.FS
}

// New returns a handler serving dir. It refuses a directory without an
// index.html, so a misconfigured CRITICAL_WEB_DIR stops the server at boot
// instead of answering 404 to every page.
func New(dir string) (*Handler, error) {
	fsys := os.DirFS(dir)
	st, err := fs.Stat(fsys, indexFile)
	if err != nil {
		return nil, fmt.Errorf("web dir %q: %w", dir, err)
	}
	if st.IsDir() {
		return nil, fmt.Errorf("web dir %q: %s is a directory", dir, indexFile)
	}
	return &Handler{fsys: fsys}, nil
}

// ServeHTTP answers GET and HEAD with the requested file, or index.html when
// the path names no file. Hashed assets are cacheable forever; everything else
// (index.html first) is revalidated on every load so a deploy is picked up.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	// path.Clean on a rooted path drops every "..", so name stays inside fsys.
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = indexFile
	}
	if h.serveFile(w, r, name) {
		return
	}
	if !h.serveFile(w, r, indexFile) {
		http.Error(w, "web app not available", http.StatusInternalServerError)
	}
}

// serveFile writes the regular file name and reports whether it existed.
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	f, err := h.fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	cache := cacheRevalidate
	if strings.HasPrefix(name, assetsDir) {
		cache = cacheImmutable
	}
	w.Header().Set("Cache-Control", cache)
	http.ServeContent(w, r, name, st.ModTime(), rs)
	return true
}

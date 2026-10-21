// Package changeloghdl exposes GET /api/changelog (ADR 0039): the versions of the running build with
// their Features and Fixes, newest first, paginated by version. It needs a session like any route that
// is not listed in middleware.Public.
package changeloghdl

import (
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/changelog"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

// RegisterRoutes wires the changelog read onto r. A nil log is the empty changelog.
func RegisterRoutes(r Router, log *changelog.Log) {
	r.Get("/api/changelog", render.Handle(Get(log)))
}

// VersionDTO is one version: its Features and Fixes are lists (empty, never null).
type VersionDTO struct {
	Version  string   `json:"version"`
	Features []string `json:"features"`
	Fixes    []string `json:"fixes"`
}

// PageDTO is the answer. NextCursor is absent on the last page; it goes back as ?cursor=.
type PageDTO struct {
	Versions   []VersionDTO `json:"versions"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// Get is GET /api/changelog?cursor=&limit=. limit is 1 to changelog.MaxLimit (more is the maximum;
// absent is changelog.DefaultLimit); a limit that is not a positive integer or a cursor that names no
// version is a 400.
func Get(log *changelog.Log) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.URL.Query()
		limit := 0
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				return domain.FieldErrorf("limit", "%q is not a positive number", raw)
			}
			limit = n
		}
		page, err := log.Page(q.Get("cursor"), limit)
		if err != nil {
			return err
		}
		out := PageDTO{Versions: make([]VersionDTO, len(page.Versions)), NextCursor: page.Next}
		for i, v := range page.Versions {
			out.Versions[i] = VersionDTO{Version: v.Version, Features: v.Features, Fixes: v.Fixes}
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

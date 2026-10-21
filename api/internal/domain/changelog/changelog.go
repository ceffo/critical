// Package changelog is the in-app changelog (ADR 0039): per release version, the Features and the
// Fixes, newest version first. It is a build artifact generated from the commit titles
// (scripts/changelog.sh) and baked into the image, so it is read-only data with no storage; its
// strings are English as they are (ADR 0039 step 5).
package changelog

import "github.com/ceffo/critical/api/internal/domain"

// Version is what one release version changed for the user (ADR 0039 step 4): both lists are in the
// order of the commits, and a version with neither is still a Version, with both empty.
type Version struct {
	Version  string
	Features []string
	Fixes    []string
}

// DefaultLimit is the page size when the request names none; MaxLimit is the stable cap on a page.
const (
	DefaultLimit = 20
	MaxLimit     = 50
)

// Log is the changelog of one build, newest version first. The zero value and nil are the empty
// changelog of a dev build, which has no tag history.
type Log struct {
	versions []Version
}

// New is the changelog of versions, given newest first. It refuses a version named twice or not at
// all: the cursor of a page is a version.
func New(versions []Version) (*Log, error) {
	seen := make(map[string]bool, len(versions))
	for _, v := range versions {
		switch {
		case v.Version == "":
			return nil, domain.FieldErrorf("version", "a changelog version has no name")
		case seen[v.Version]:
			return nil, domain.FieldErrorf("version", "changelog version %q twice", v.Version)
		}
		seen[v.Version] = true
	}
	return &Log{versions: versions}, nil
}

// Page is a run of versions and the cursor of the next run ("" at the end).
type Page struct {
	Versions []Version
	Next     string
}

// Page returns up to limit versions after the version named by cursor (from the newest when cursor
// is empty). A limit of 0 is DefaultLimit, one above MaxLimit is MaxLimit. A negative limit or a
// cursor naming no version is a domain.FieldError. Safe on a nil Log.
func (l *Log) Page(cursor string, limit int) (Page, error) {
	switch {
	case limit < 0:
		return Page{}, domain.FieldErrorf("limit", "limit must be positive")
	case limit == 0:
		limit = DefaultLimit
	case limit > MaxLimit:
		limit = MaxLimit
	}
	var all []Version
	if l != nil {
		all = l.versions
	}
	start := 0
	if cursor != "" {
		found := false
		for i, v := range all {
			if v.Version == cursor {
				start, found = i+1, true
				break
			}
		}
		if !found {
			return Page{}, domain.FieldErrorf("cursor", "cursor %q is not a version of the changelog", cursor)
		}
	}
	end := min(start+limit, len(all))
	p := Page{Versions: all[start:end]}
	if end < len(all) {
		p.Next = all[end-1].Version
	}
	return p, nil
}

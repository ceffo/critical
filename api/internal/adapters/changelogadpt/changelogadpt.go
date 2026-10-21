// Package changelogadpt reads the changelog baked into the image (ADR 0039 step 6): the JSON file
// scripts/changelog.sh made at the release build, at CRITICAL_CHANGELOG. The server does no git work.
package changelogadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ceffo/critical/api/internal/domain/changelog"
)

type fileDTO struct {
	Versions []versionDTO `json:"versions"`
}

type versionDTO struct {
	Version  string   `json:"version"`
	Features []string `json:"features"`
	Fixes    []string `json:"fixes"`
}

// Load reads the changelog file. An empty path is the empty changelog (a dev build has no tag
// history, so no file); a path that cannot be read, or a file that is not the generator's output, is
// an error: an image that names its changelog must carry it whole.
func Load(path string) (*changelog.Log, error) {
	if path == "" {
		return changelog.New(nil)
	}
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own setting
	if err != nil {
		return nil, fmt.Errorf("changelog: %w", err)
	}
	return Parse(raw)
}

// Parse reads the generator's JSON.
func Parse(raw []byte) (*changelog.Log, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var f fileDTO
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("changelog: %w", err)
	}
	versions := make([]changelog.Version, len(f.Versions))
	for i, v := range f.Versions {
		versions[i] = changelog.Version{Version: v.Version, Features: nonNil(v.Features), Fixes: nonNil(v.Fixes)}
	}
	return changelog.New(versions)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

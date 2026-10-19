// Package journalfixture reads the web journal's fixtures in tests
// (web/src/journal/fixtures/<kind>.json): the entries of each kind as
// GET /api/log serves them, which the web's per-kind tests word in English and
// French. The owning service of each kind proves in its own tests that its
// fixtures are what it logs, so the web tests run on real records.
//
// The path to the fixtures is written here and nowhere else in Go.
package journalfixture

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// relDir is the fixtures' directory, relative to this file.
const relDir = "../../../../../web/src/journal/fixtures"

// Case is one fixture entry of a kind.
type Case struct {
	Name string
	// Roll is the flag the fixture gives the entry, as GET /api/log marks it.
	Roll bool
	// Payload is the logged payload; Effective the payload after an amend
	// (nil when the fixture has none).
	Payload, Effective json.RawMessage
}

type file map[string]struct {
	Roll             bool            `json:"roll"`
	Payload          json.RawMessage `json:"payload"`
	EffectivePayload json.RawMessage `json:"effectivePayload"`
}

// Dir is the absolute path of the fixtures directory.
func Dir(t testing.TB) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(here), relDir)
}

// Kinds are the kinds that have a fixture file, sorted.
func Kinds(t testing.TB) []actionlog.Kind {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(Dir(t), "*.json"))
	require.NoError(t, err)
	kinds := make([]actionlog.Kind, 0, len(files))
	for _, f := range files {
		kinds = append(kinds, actionlog.Kind(strings.TrimSuffix(filepath.Base(f), ".json")))
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return kinds
}

// Cases are the fixtures of kind k, by name; the test fails when there are none.
func Cases(t testing.TB, k actionlog.Kind) []Case {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(Dir(t), string(k)+".json"))
	require.NoError(t, err, "no fixture for kind %s", k)
	var f file
	require.NoError(t, json.Unmarshal(raw, &f))
	require.NotEmpty(t, f, "fixture of kind %s has no case", k)
	names := make([]string, 0, len(f))
	for n := range f {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Case, 0, len(names))
	for _, n := range names {
		c := f[n]
		require.NotEmpty(t, c.Payload, "%s/%s: no payload", k, n)
		out = append(out, Case{Name: n, Roll: c.Roll, Payload: c.Payload, Effective: c.EffectivePayload})
	}
	return out
}

// UpdateEnv is the environment variable that, set to 1, makes Matches rewrite the fixture files with what the
// records write instead of failing on a difference:
//
//	UPDATE_JOURNAL_FIXTURES=1 go test -count=1 ./internal/domain/... -run TestJournalFixtures
//
// (just journal-fixtures, which also runs prettier). Review the diff: the web tests must still pass.
const UpdateEnv = "UPDATE_JOURNAL_FIXTURES"

// Matches fails when a payload of kind k's fixtures is not exactly what the
// service logs: record is the type the service marshals into the entry. The
// payload is read strictly (a field the record lacks fails) and written back;
// the result must equal the fixture, so a field the record gained, lost or
// renamed, or a value it writes differently, fails too.
func Matches[T any](t *testing.T, k actionlog.Kind) {
	t.Helper()
	MatchesAs(t, k, func(raw json.RawMessage) (json.RawMessage, error) {
		var rec T
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return nil, err
		}
		return marshal(rec)
	})
}

// MatchesAs is Matches for a kind whose served payload is a function of the
// record: serve turns a fixture payload into what the service would serve
// for it.
func MatchesAs(t *testing.T, k actionlog.Kind, serve func(json.RawMessage) (json.RawMessage, error)) {
	t.Helper()
	update := os.Getenv(UpdateEnv) == "1"
	path := filepath.Join(Dir(t), string(k)+".json")
	raw, err := os.ReadFile(path) //nolint:gosec // a checked-in fixture, path built from a constant directory and a kind
	require.NoError(t, err, "no fixture for kind %s", k)
	var f map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &f))
	require.NotEmpty(t, f, "fixture of kind %s has no case", k)

	names := make([]string, 0, len(f))
	for n := range f {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, field := range []string{"payload", "effectivePayload"} {
			p, ok := f[n][field]
			if !ok {
				require.NotEqual(t, "payload", field, "%s/%s: no payload", k, n)
				continue
			}
			t.Run(string(k)+"/"+n+"/"+field, func(t *testing.T) {
				got, err := serve(p)
				require.NoError(t, err, "the fixture is not what the %s record reads", k)
				if update {
					f[n][field] = got
					return
				}
				require.JSONEq(t, string(p), string(got),
					"the %s record no longer writes this fixture (%s=1 with go test -count=1 rewrites it)", k, UpdateEnv)
			})
		}
	}
	if update && !t.Failed() {
		out, err := marshalIndent(f)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(out, '\n'), 0o600))
	}
}

func marshal(v any) (json.RawMessage, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b.Bytes()), nil
}

func marshalIndent(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b.Bytes()), nil
}

// RollFlagsMatch fails when a fixture's roll flag differs from actionlog.IsRoll.
func RollFlagsMatch(t *testing.T) {
	t.Helper()
	for _, k := range Kinds(t) {
		for _, c := range Cases(t, k) {
			require.Equalf(t, actionlog.IsRoll(k), c.Roll, "%s/%s: roll flag of the fixture", k, c.Name)
		}
	}
}

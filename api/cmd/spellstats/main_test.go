package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

const (
	fixtureCatalogue = "../../testdata/catalogue.json"
	committedStats   = "../../../web/src/lib/__fixtures__/spellStats.json"
)

// The web's statText fixture test reads a committed copy of this command's output over the
// fixture catalogue. If the parser changes and the copy is not regenerated, that web test keeps
// passing on stale data: this fails instead. The comparison is on the decoded JSON, so the
// committed file's formatting (prettier) does not matter.
func TestCommittedWebFixtureMatchesParser(t *testing.T) {
	spells, err := build(fixtureCatalogue)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := json.Marshal(spells)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(committedStats)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(committed, &want); err != nil {
		t.Fatalf("%s: %v", committedStats, err)
	}
	if err := json.Unmarshal(generated, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("web/src/lib/__fixtures__/spellStats.json is stale: the spell stats parser no longer produces it.\n" +
			"Regenerate it with `just spell-stats-fixture`, then `cd web && npx vitest run -u src/lib/statTextFixture.test.ts` " +
			"if the worded snapshot changes on purpose, and commit both.")
	}
}

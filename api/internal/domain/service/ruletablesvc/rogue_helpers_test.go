package ruletablesvc_test

import (
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// rogue is the fixture class's rogue_skills params (ADR 0040 step 3), nil
// for a class without them.
func rogue(t testing.TB, class string) *classes.RogueSkills {
	t.Helper()
	def, ok := cataloguetest.Definitions(t).Class(class)
	if !ok {
		t.Fatalf("fixture class %q", class)
	}
	f, ok := def.Lists(classes.KindRogueSkills)
	if !ok {
		return nil
	}
	return f.RogueSkills
}

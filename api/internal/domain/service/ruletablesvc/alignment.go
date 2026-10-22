package ruletablesvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// Alignments are the nine alignment codes of the PHB (chapter 4), in the
// order the book lists them (ADR 0032 step 2).
var Alignments = classes.Alignments

// AlignmentsOf are the alignments class admits (its definition's, ADR 0040
// step 1.5), in Alignments order; none for a class the catalogue does not
// define.
func (t *Tables) AlignmentsOf(class string) []string {
	c, err := t.Class(class)
	if err != nil {
		return []string{}
	}
	return slices.Clone(c.Alignments)
}

// AlignmentAllowed reports whether class admits alignment (ADR 0040 step
// 1.5); false for a class the catalogue does not define.
func (t *Tables) AlignmentAllowed(class, alignment string) bool {
	c, err := t.Class(class)
	return err == nil && c.AlignmentAllowed(alignment)
}

package ruletablesvc

import "slices"

// Alignments are the nine alignment codes of the PHB (chapter 4), in the
// order the book lists them (ADR 0032 step 2).
var Alignments = []string{
	"lawful_good", "neutral_good", "chaotic_good",
	"lawful_neutral", "true_neutral", "chaotic_neutral",
	"lawful_evil", "neutral_evil", "chaotic_evil",
}

// alignmentLimits are the classes whose alignment the book limits (PHB
// chapter 3) and the alignments each admits, in Alignments order. A class
// not listed admits all nine: the fighter (any), the mage (no limit stated)
// and the cleric, whose alignment the book ties to the deity's, which the app
// does not model.
var alignmentLimits = map[string][]string{
	// Bard: "always partially neutral", so the five alignments with a
	// neutral part.
	"bard": {"neutral_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "neutral_evil"},
	// Thief: any but lawful good.
	"thief": {
		"neutral_good", "chaotic_good", "lawful_neutral", "true_neutral", "chaotic_neutral",
		"lawful_evil", "neutral_evil", "chaotic_evil",
	},
}

// AlignmentsOf are the alignments class admits, in Alignments order.
func AlignmentsOf(class string) []string {
	if admitted, ok := alignmentLimits[class]; ok {
		return slices.Clone(admitted)
	}
	return slices.Clone(Alignments)
}

// AlignmentAllowed reports whether class admits alignment (a code of
// Alignments).
func AlignmentAllowed(class, alignment string) bool {
	if admitted, ok := alignmentLimits[class]; ok {
		return slices.Contains(admitted, alignment)
	}
	return slices.Contains(Alignments, alignment)
}

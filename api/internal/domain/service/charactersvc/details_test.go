package charactersvc_test

import (
	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

func str(s string) *string { return &s }

// ADR 0032 step 2: the nine PHB alignments are the codes; a class limit is a refusal of the set
// (creation and edit apply the same Details.Apply). Per class: the bard refuses each of the four
// alignments without a neutral part and accepts the five others, the thief refuses lawful good
// only, the fighter, the mage and the cleric accept the nine.
func TestApply_Step2_AlignmentClassLimit(t *testing.T) {
	refused := map[string][]string{
		"bard":    {"lawful_good", "chaotic_good", "lawful_evil", "chaotic_evil"},
		"thief":   {"lawful_good"},
		"fighter": nil, "mage": nil, "cleric": nil,
	}
	require.Len(t, ruletablesvc.Alignments, 9)
	for class, no := range refused {
		for _, alignment := range ruletablesvc.Alignments {
			t.Run(class+" "+alignment, func(t *testing.T) {
				got, err := charactersvc.Details{}.Apply(aligns(t, class), charactersvc.DetailsPatch{Alignment: str(alignment)})
				if contains(no, alignment) {
					var fe *domain.FieldError
					require.ErrorAs(t, err, &fe)
					assert.Equal(t, "alignment", fe.Field)
					assert.Equal(t, "alignment_not_allowed", fe.Message)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, alignment, got.Alignment)
			})
		}
		assert.Len(t, aligns(t, class), 9-len(no), class, "the options list the admitted ones")
	}
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

// ADR 0032 steps 2 and 3: an empty or unknown code is refused on the edit, an edit that does not
// carry them leaves them, and a value that is refused changes nothing.
func TestApply_Step2And3_EmptyUnknownAndUntouched(t *testing.T) {
	stored := charactersvc.Details{Alignment: "neutral_good", Sex: "male"}
	for _, tt := range []struct {
		name        string
		p           charactersvc.DetailsPatch
		field, code string
	}{
		{"empty alignment", charactersvc.DetailsPatch{Alignment: str("")}, "alignment", "empty"},
		{"blank alignment", charactersvc.DetailsPatch{Alignment: str("  ")}, "alignment", "empty"},
		{"unknown alignment", charactersvc.DetailsPatch{Alignment: str("Chaotique neutre")}, "alignment", "unknown"},
		{"empty sex", charactersvc.DetailsPatch{Sex: str("")}, "sex", "empty"},
		{"unknown sex", charactersvc.DetailsPatch{Sex: str("M")}, "sex", "unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stored.Apply(aligns(t, "bard"), tt.p)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.Equal(t, stored, got)
		})
	}
	got, err := stored.Apply(aligns(t, "bard"), charactersvc.DetailsPatch{Hair: str("black"), Sex: str("female")})
	require.NoError(t, err)
	assert.Equal(t, charactersvc.Details{Alignment: "neutral_good", Sex: "female", Hair: "black", Languages: []string{}}, got)
}

// ADR 0032 step 8.2 and 8.4: a migrated character lacking a value reads it as missing, and an
// alignment its class refuses is kept and read as a warning; setting another detail on it is not
// refused (the check is of a value being set).
func TestDetails_Step8_MissingAndKeptAlignmentWarning(t *testing.T) {
	assert.Equal(t, []string{"alignment", "sex"}, charactersvc.Details{}.Missing())
	assert.Equal(t, []string{"sex"}, charactersvc.Details{Alignment: "true_neutral"}.Missing())
	assert.Empty(t, charactersvc.Details{Alignment: "true_neutral", Sex: "male"}.Missing())

	thief := charactersvc.Details{Alignment: "lawful_good", Sex: "male"}
	assert.Equal(t, []charactersvc.Warning{{Code: "alignment_not_allowed", Field: "alignment"}}, thief.Warnings(aligns(t, "thief")))
	assert.Empty(t, thief.Warnings(aligns(t, "fighter")))
	assert.Empty(t, charactersvc.Details{}.Warnings(aligns(t, "thief")), "a missing value is no warning")

	got, err := thief.Apply(aligns(t, "thief"), charactersvc.DetailsPatch{Eyes: str("grey")})
	require.NoError(t, err)
	assert.Equal(t, "lawful_good", got.Alignment)
}

func TestApply_Step8_KeptForbiddenAlignmentDoesNotBlockOtherEdits(t *testing.T) {
	thief := charactersvc.Details{Alignment: "lawful_good", Sex: "male"}
	got, err := thief.Apply(aligns(t, "thief"), charactersvc.DetailsPatch{Alignment: str("lawful_good"), Hair: str("red")})
	require.NoError(t, err, "the unchanged alignment passes")
	assert.Equal(t, "red", got.Hair)

	neutral := charactersvc.Details{Alignment: "lawful_good"}
	_, err = neutral.Apply(aligns(t, "bard"), charactersvc.DetailsPatch{Alignment: str("lawful_evil")})
	require.Error(t, err, "a change to a forbidden alignment stays refused")
}

func TestDetails_Step8_ANonCodeReadsAsMissing(t *testing.T) {
	d := charactersvc.Details{Alignment: "Chaotic Good", Sex: "Homme"}
	assert.Equal(t, []string{"alignment", "sex"}, d.Missing())
	assert.Empty(t, d.Warnings(aligns(t, "thief")), "no warning for a value that is not a code")
	assert.Equal(t, charactersvc.Details{}, d.Coded())
}

// ADR 0040 step 1.5: every class of the catalogue lists the alignments it admits.
func TestEveryClassHasADecidedAlignmentList(t *testing.T) {
	for _, c := range cataloguetest.Definitions(t).Classes() {
		assert.NotEmpty(t, c.Alignments, c.ID)
	}
}

// aligns are the alignments the fixture catalogue's class admits.
func aligns(t *testing.T, class string) []string {
	t.Helper()
	return charactersvc.AlignmentsOf(cataloguetest.Definitions(t), class)
}

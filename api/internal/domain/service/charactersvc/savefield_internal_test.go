package charactersvc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// Every shared save category code names its own Saves field, in the order of the list.
func TestSaveField_CoversSaveCategories(t *testing.T) {
	var s Saves
	for _, cat := range classes.SaveCategories {
		p := saveField(&s, cat)
		if assert.NotNil(t, p, cat) {
			*p = 1
		}
	}
	assert.Equal(t, Saves{ParaPoison: 1, RodStaffWand: 1, PetriPoly: 1, Breath: 1, Spell: 1}, s)
	assert.Nil(t, saveField(&s, "poison"))
}

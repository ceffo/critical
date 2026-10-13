package spellsvc_test

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	refs := []spellsvc.Ref{
		{Name: "Fortify", Class: "Priest", Level: 3},
		{Name: "Fortify", Class: "Priest", Level: 4},
		{Name: "Hold Person", Class: "Wizard", Level: 3},
		{Name: "Hold Person", Class: "Priest", Level: 2},
		{Name: "Melf's Acid Arrow", Class: "Wizard", Level: 2},
		{Name: "Twin", Class: "Wizard", Level: 5},
		{Name: "Twin", Class: "Priest", Level: 1},
		{Name: "Twin", Class: "Priest", Level: 6},
	}
	assert.Equal(t, []string{
		"fortify", "fortify_l4", // same class, same name: lowest level keeps the plain key
		"hold_person", "hold_person_priest", // Wizard keeps the existing slug, Priest is qualified
		"melfs_acid_arrow",
		"twin", "twin_priest", "twin_priest_l6",
	}, spellsvc.Keys(refs))
}

func TestKeys_OrderOfTheCatalogDoesNotMatter(t *testing.T) {
	a := spellsvc.Keys([]spellsvc.Ref{{Name: "Fortify", Class: "Priest", Level: 4}, {Name: "Fortify", Class: "Priest", Level: 3}})
	assert.Equal(t, []string{"fortify_l4", "fortify"}, a)
}

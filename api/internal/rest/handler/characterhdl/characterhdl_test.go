package characterhdl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

func TestToDTO_PrimaryClass(t *testing.T) {
	frozen := classes.Entry{Class: "fighter", RealizedLevel: 9, State: classes.EntryFrozen, LeftAtLevel: 9}
	cases := []struct {
		name string
		c    charactersvc.Character
		want string
	}{
		{"single class", charactersvc.Character{Classes: []classes.Entry{classes.SingleEntry("mage", 3, 0)}}, "mage"},
		{"mirror only", charactersvc.Character{Class: "cleric", RealizedLevel: 2}, "cleric"},
		{"multi class", charactersvc.Character{Classes: []classes.Entry{
			classes.SingleEntry("fighter", 4, 0), classes.SingleEntry("mage", 5, 0)}}, "fighter"},
		{"dual class skips the frozen first entry", charactersvc.Character{Classes: []classes.Entry{
			frozen, classes.SingleEntry("mage", 10, 0)}}, "mage"},
		{"no active entry falls back to the first", charactersvc.Character{Classes: []classes.Entry{frozen}}, "fighter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, toDTO(&tc.c).PrimaryClass)
		})
	}
}

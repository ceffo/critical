package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// carryArmours are catalogue body armours (format 7, PHB Table 46's numbers).
var carryArmours = []ruletablesvc.Armour{
	{Code: "leather", Name: localized.Text{"en": "Leather armor", "fr": "Armure de cuir"}, BaseAC: 8, SkillColumn: "leather", ArmorRow: "leather"},
	{Code: "studded_leather", Name: localized.Text{"en": "Studded leather"}, BaseAC: 7, SkillColumn: "padded_studded", ArmorRow: "studded_leather"},
	{Code: "elven_chain_mail", Name: localized.Text{"en": "Elven chain mail"}, BaseAC: 5, SkillColumn: "elven_chain", ArmorRow: "chain"},
	{Code: "chain_mail", Name: localized.Text{"en": "Chain mail"}, BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain"},
}

// ADR 0033 step 14's carry-over: a thief or a bard that wears no body armour gets the armour it
// entered as a new worn piece from the catalogue (leather and none entered: Leather armour;
// padded or studded: Studded leather; elven chain: Elven chain mail; a bard's metal: Chain mail),
// at the end of the list under a free id, the guessed ones on the review list; the skills then
// read the same armour as before. An entered none, another class, or an armour worn already give
// nothing.
func TestCarrySkillArmour_Step14(t *testing.T) {
	gear := itemsvc.Item{ID: "leather", Position: 4, Category: itemsvc.CategoryGear, State: itemsvc.StateCarried}
	for _, tc := range []struct {
		class, entered, code string
		guessed              bool
	}{
		{"thief", "leather", "leather", false},
		{"thief", "", "leather", false},
		{"thief", "padded_studded_leather", "studded_leather", true},
		{"thief", "elven_chain", "elven_chain_mail", false},
		{"bard", "", "leather", false},
		{"bard", "metal", "chain_mail", true},
	} {
		t.Run(tc.class+" "+tc.entered, func(t *testing.T) {
			co, err := itemsvc.CarrySkillArmour(tc.class, tc.entered, []itemsvc.Item{gear}, carryArmours)
			require.NoError(t, err)
			require.NotNil(t, co.Add)
			p := *co.Add
			wantID := tc.code
			if tc.code == "leather" {
				wantID = "leather_2" // the gear piece has the id already
			}
			assert.Equal(t, []any{wantID, tc.code, 5, 1, itemsvc.CategoryArmour, itemsvc.StateWorn},
				[]any{p.ID, p.CatalogueCode, p.Position, p.Count, p.Category, p.State})
			require.NoError(t, p.Validate(), "a piece the editor would accept")
			entered := tc.entered
			if entered == "" {
				entered = ruletablesvc.ArmorLeather
			}
			assert.Equal(t, entered, ruletablesvc.SkillArmorFor(tc.class, itemsvc.WornSkillColumn([]itemsvc.Item{gear, p})),
				"every skill percentage the same before and after")
			if tc.guessed {
				assert.Equal(t, &itemsvc.Review{ItemID: wantID, Reason: itemsvc.ReasonArmourGuessed}, co.Review)
			} else {
				assert.Nil(t, co.Review)
			}
		})
	}
	t.Run("nothing to carry", func(t *testing.T) {
		wornPlate := itemsvc.Item{ID: "plate", Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: "none_allowed"}
		for _, tc := range []struct {
			class, entered string
			pieces         []itemsvc.Item
		}{
			{"thief", "none", nil},
			{"fighter", "", nil},
			{"thief", "leather", []itemsvc.Item{wornPlate}},
		} {
			co, err := itemsvc.CarrySkillArmour(tc.class, tc.entered, tc.pieces, carryArmours)
			require.NoError(t, err)
			assert.Equal(t, itemsvc.CarryOver{}, co, "%s %q", tc.class, tc.entered)
		}
	})
	t.Run("a carried piece of the entered column is worn", func(t *testing.T) {
		pieces := []itemsvc.Item{
			{ID: "studded", Category: itemsvc.CategoryArmour, State: itemsvc.StateCarried, SkillColumn: "padded_studded"},
			{ID: "my_chain", Category: itemsvc.CategoryArmour, State: itemsvc.StateCarried, SkillColumn: "metal"},
		}
		co, err := itemsvc.CarrySkillArmour("bard", "metal", pieces, carryArmours)
		require.NoError(t, err)
		assert.Equal(t, itemsvc.CarryOver{Wear: "my_chain"}, co)
	})
	t.Run("the catalogue lacks the row", func(t *testing.T) {
		_, err := itemsvc.CarrySkillArmour("thief", "leather", nil, nil)
		require.ErrorIs(t, err, itemsvc.ErrCarryOverArmourMissing)
		_, err = itemsvc.CarrySkillArmour("thief", "plate", nil, carryArmours)
		require.Error(t, err, "an entered value the column never held")
	})
}

// ADR 0033 step 14: the skills read the worn body armour's column; a carried armour and a worn
// shield are no worn body armour.
func TestWornSkillColumn_Step14(t *testing.T) {
	assert.Empty(t, itemsvc.WornSkillColumn(nil))
	pieces := []itemsvc.Item{
		{ID: "shield", Category: itemsvc.CategoryShield, State: itemsvc.StateWorn},
		{ID: "chain", Category: itemsvc.CategoryArmour, State: itemsvc.StateCarried, SkillColumn: "metal"},
	}
	assert.Empty(t, itemsvc.WornSkillColumn(pieces))
	pieces = append(pieces, itemsvc.Item{ID: "leather", Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: "leather"})
	assert.Equal(t, "leather", itemsvc.WornSkillColumn(pieces))
}

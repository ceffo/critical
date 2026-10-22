package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// catalogueArmour are catalogue body armours (format 7, PHB Table 46's numbers) the skill armour
// carry-over gives in these tests.
var catalogueArmour = []ruletablesvc.Armour{
	{Code: "leather", Name: localized.Text{"en": "Leather armor", "fr": "Armure de cuir"}, BaseAC: 8, SkillColumn: "leather", ArmorRow: "leather"},
	{Code: "studded_leather", Name: localized.Text{"en": "Studded leather", "fr": "Cuir clouté"}, BaseAC: 7, SkillColumn: "padded_studded", ArmorRow: "studded_leather"},
	{Code: "elven_chain_mail", Name: localized.Text{"en": "Elven chain mail", "fr": "Cotte de mailles elfique"}, BaseAC: 5, SkillColumn: "elven_chain", ArmorRow: "chain"},
	{Code: "chain_mail", Name: localized.Text{"en": "Chain mail", "fr": "Cotte de mailles"}, BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain"},
	{Code: "plate_mail", Name: localized.Text{"en": "Plate mail", "fr": "Harnois"}, BaseAC: 3, SkillColumn: "none_allowed", ArmorRow: "plate"},
}

// catalogueEquipment is the catalogue's equipment these tests migrate and seed with.
var catalogueEquipment = ruletablesvc.Equipment{Weapons: catalogueWeapons, Armour: catalogueArmour}

// skillArmourCharacter stores a character of class with the skill armour skillArmor entered and
// the pieces given, and returns its id.
func skillArmourCharacter(t *testing.T, f *fakeStore, class, skillArmor string, pieces ...storeq.InsertItemParams) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := f.InsertCharacter(ctx, storeq.InsertCharacterParams{Name: class + " " + skillArmor})
	require.NoError(t, err)
	f.t.Classes[id], f.t.SkillArmors[id] = class, skillArmor
	for _, p := range pieces {
		key, err := addSeedText(ctx, f, localized.Of(p.ID))
		require.NoError(t, err)
		p.CharacterID, p.Name, p.Behaviour, p.Count, p.Powers, p.DamageTypes = id, key, "stock", 1, "[]", []string{}
		require.NoError(t, f.InsertItem(ctx, p))
	}
	return id
}

// carry runs the data step after migration 020 on f with the catalogue's armours.
func carry(t *testing.T, f *fakeStore, equipment ruletablesvc.Equipment) error {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), equipment, catalogueDefs)
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return after[skillArmourMigration](ctx, f)
	})
}

// worn is the character's worn body armour, ok false when it wears none.
func worn(f *fakeStore, owner int64) (storeq.InsertItemParams, bool) {
	for _, it := range f.t.Items {
		if it.CharacterID == owner && it.Category == "armour" && it.State == "worn" {
			return it, true
		}
	}
	return storeq.InsertItemParams{}, false
}

// ADR 0033 step 14's carry-over, the data step after migration 020: a thief or a bard that wears
// no body armour gets the armour it entered, worn, from the catalogue (named in every language
// of the row): leather and none entered give Leather armour, padded or studded leather Studded
// leather, elven chain Elven chain mail, a bard's metal Chain mail, none nothing; the guessed
// pieces (studded, chain) are on the review list. Each skill reads the same Table 29 armour
// before and after (the entered value, none entered as leather).
func TestCarrySkillArmour_Step14_EachEnteredArmourIsWorn(t *testing.T) {
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
		{"thief", "none", "", false},
		{"fighter", "", "", false},
	} {
		t.Run(fmt.Sprintf("%s %q", tc.class, tc.entered), func(t *testing.T) {
			f := newFakeStore()
			owner := skillArmourCharacter(t, f, tc.class, tc.entered,
				storeq.InsertItemParams{ID: "rope", Position: 2, Category: "gear", State: "carried"})
			require.NoError(t, carry(t, f, catalogueEquipment))
			got, ok := worn(f, owner)
			if tc.code == "" {
				assert.False(t, ok, "nothing worn")
				assert.Len(t, f.t.Items, 1)
				return
			}
			require.True(t, ok)
			assert.Equal(t, []any{tc.code, tc.code, int64(3), int64(1), "stock"},
				[]any{got.ID, got.CatalogueCode, got.Position, got.Count, got.Behaviour}, "at the end of the list, one of it")
			var row ruletablesvc.Armour
			for _, a := range catalogueArmour {
				if a.Code == tc.code {
					row = a
				}
			}
			assert.Equal(t, []any{int64(row.BaseAC), row.SkillColumn, row.ArmorRow}, []any{got.BaseAc, got.SkillColumn, got.ArmorRow})
			assert.Equal(t, row.Name, f.t.Texts[got.Name], "named in every language of the catalogue row")
			entered := tc.entered
			if entered == "" {
				entered = ruletablesvc.ArmorLeather
			}
			assert.Equal(t, entered, ruletablesvc.SkillArmorFor(rogueSkillsOf(t, tc.class), got.SkillColumn), "the skills read the same armour")
			review := f.t.Reviews[fmt.Sprintf("%d|%s|%s", owner, tc.code, itemsvc.ReasonArmourGuessed)]
			assert.Equal(t, tc.guessed, review, "a guessed armour is on the review list")
		})
	}
}

// ADR 0033 step 14: a thief that carries an armour of the column it entered puts that piece on
// (nothing is added); one that already wears a body armour keeps it (its skills follow it).
func TestCarrySkillArmour_Step14_ACarriedPieceIsWornAndAWornOneKept(t *testing.T) {
	f := newFakeStore()
	carrying := skillArmourCharacter(t, f, "thief", "elven_chain",
		storeq.InsertItemParams{ID: "plate", Position: 0, Category: "armour", State: "carried", SkillColumn: "none_allowed", ArmorRow: "plate"},
		storeq.InsertItemParams{ID: "my_chain", Position: 1, Category: "armour", State: "carried", SkillColumn: "elven_chain", ArmorRow: "chain"})
	wearing := skillArmourCharacter(t, f, "thief", "leather",
		storeq.InsertItemParams{ID: "plate", Position: 0, Category: "armour", State: "worn", SkillColumn: "none_allowed", ArmorRow: "plate"})
	require.NoError(t, carry(t, f, catalogueEquipment))

	got, ok := worn(f, carrying)
	require.True(t, ok)
	assert.Equal(t, "my_chain", got.ID, "the carried piece of the entered column, not the first armour")
	assert.Equal(t, "carried", f.t.Items[rowKey(carrying, "plate")].State)
	got, ok = worn(f, wearing)
	require.True(t, ok)
	assert.Equal(t, "plate", got.ID)
	assert.Len(t, f.t.Items, 3, "nothing added")
	assert.Empty(t, f.t.Reviews)
}

// The carry-over runs once (migration 020's data step), so it refuses a catalogue without the
// armour it needs (before format 7): the run fails and rolls back, and the next start retries it.
// With no thief or bard to carry over, no catalogue is needed.
func TestCarrySkillArmour_Step14_RefusedWithoutTheCatalogueArmour(t *testing.T) {
	f := newFakeStore()
	skillArmourCharacter(t, f, "fighter", "")
	require.NoError(t, carry(t, f, ruletablesvc.Equipment{}))

	owner := skillArmourCharacter(t, f, "thief", "leather")
	err := carry(t, f, ruletablesvc.Equipment{})
	require.ErrorIs(t, err, itemsvc.ErrCarryOverArmourMissing)
	_, ok := worn(f, owner)
	assert.False(t, ok, "rolled back")
	assert.Empty(t, f.t.Items)
}

// A rerun of the step (a second migration run never runs it, but a seed after it) adds nothing:
// the character wears its armour already.
func TestCarrySkillArmour_Step14_Idempotent(t *testing.T) {
	f := newFakeStore()
	skillArmourCharacter(t, f, "bard", "metal")
	require.NoError(t, carry(t, f, catalogueEquipment))
	require.NoError(t, carry(t, f, catalogueEquipment))
	assert.Len(t, f.t.Items, 1)
	assert.Len(t, f.t.Reviews, 1)
}

// rogueSkillsOf is the fixture class's rogue_skills params (ADR 0040 step 3).
func rogueSkillsOf(t *testing.T, class string) *classes.RogueSkills {
	t.Helper()
	def, ok := catalogueDefs.Class(class)
	require.True(t, ok, class)
	f, ok := def.Lists(classes.KindRogueSkills)
	require.True(t, ok, class)
	return f.RogueSkills
}

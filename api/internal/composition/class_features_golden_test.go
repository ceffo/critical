package composition_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0040 step 11.3 (no rule moves for the five classes): with the class
// features read from each class's feature list, the five classes derive
// exactly what they derived when Go lists of class ids granted them. The
// golden file was written by this test on the code before the move (the
// lists, the spell-slot switch and the skill-budget constants) and is
// compared as is: the spell slots (Tables 21, 24 with Table 5, 32), the
// spell failure (Table 5), the turning (Table 61), the thief and bard skills
// with their armour, budget and points (Tables 26 to 29, 33), the backstab
// (Table 30), the specialization flag, the spellbook's maximum spell level
// and the attacks per round of a held long sword (Table 15), at levels 1, 7
// and 13 (the warrior's rate steps), unarmoured with Wisdom 17 and in chain
// mail with Wisdom 10 (the bard's metal armour, the thief's worst column,
// the cleric's spell failure). The exceptional Strength
// flag of the derived block is new and left out; its own tests cover it.
func TestClasses_Golden_FeaturesDeriveWhatTheGoListsDid(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	points := map[string]map[string]int{
		"thief": {"pick_pockets": 30, "open_locks": 20, "climb_walls": 10},
		"bard":  {"climb_walls": 10, "detect_noise": 10},
	}
	out := map[string]any{}
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		for _, level := range []int{1, 7, 13} {
			for _, armour := range []string{"", "chain_mail"} {
				wis := 17 // Table 5's bonus spells
				if armour != "" {
					wis = 10 // Table 5's spell failure
				}
				id, err := m.AddCharacter(ctx, charactersvc.Character{
					Name: "Golden", Class: class, Race: "human", Str: 18, Dex: 17, Con: 14, Int: 17, Wis: wis, Cha: 16,
					HPMax: 20, RealizedLevel: level, SkillPoints: points[class],
				})
				require.NoError(t, err)
				_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
				require.NoError(t, err)
				if armour != "" {
					_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: armour, Equip: true})
					require.NoError(t, err)
				}

				got, err := bs.Character.GetCharacter(ctx, id)
				require.NoError(t, err)
				form, err := bs.Combat.GetAttackForm(ctx, id)
				require.NoError(t, err)
				var rate any
				if form.AttackRate != nil {
					r := *form.AttackRate
					r.ThisRound = 0
					rate = r
				}
				out[class+" level "+strconv.Itoa(level)+" "+orNone(armour)+" wis "+strconv.Itoa(wis)] = map[string]any{
					"derived": withoutNewFields(t, got.Derived), "attackRate": rate,
				}
			}
		}
	}
	body, err := json.Marshal(out)
	require.NoError(t, err)
	golden(t, "class_features.json", body)
}

func orNone(armour string) string {
	if armour == "" {
		return "unarmoured"
	}
	return armour
}

// withoutNewFields is the derived block as JSON without the fields the
// feature lists added (exceptionalStrength), so the golden compares what
// existed before.
func withoutNewFields(t *testing.T, d charactersvc.Derived) map[string]any {
	t.Helper()
	b, err := json.Marshal(d)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	delete(m, "ExceptionalStrength")
	delete(m, "exceptionalStrength")
	return m
}

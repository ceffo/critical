package composition_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0040 steps 11.3 and 11.4 (no rule moves for the five classes and six races): with the
// equipment limits read from each class's definition and the race traits from each race's, the
// classes and races give exactly what the Go tables gave. The golden file was written by this
// test on the code before the move (itemsvc's classRules, ruletablesvc's classWeaponTypes,
// charactersvc's conSaveRaces, the identity literal) and is compared as is, with one change:
// the thief's weapon types no longer hold other_thrown, which only the lasso of its PHB list
// (p.39) gave it, and the lasso is in no catalogue weapon (ADR 0040 step 1.6). It holds, for
// every class: the class warnings of three sets of pieces worn and wielded together (armour,
// shield and weapons of the book's lists, in and out of them); for every class and race: the
// saves and the race's save bonus at Constitution 16 (Table 9); and the creation options
// (every class's weapon types, every race's Tables 10 and 11 figures).
func TestRacesAndEquipment_Golden_DeriveWhatTheGoTablesDid(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	opts, err := bs.Character.CreationOptions(ctx)
	require.NoError(t, err)
	pieces := map[string][]string{
		"mail and sword":     {"chain_mail", "medium_shield", "long_sword"},
		"leather and blunt":  {"leather", "club", "sling", "dagger", "long_bow"},
		"elven chain, staff": {"elven_chain_mail", "quarterstaff", "short_sword", "javelin", "footmans_mace"},
	}
	out := map[string]any{"options": opts}
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		for name, codes := range pieces {
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Golden", Class: class, Race: "human", Str: 16, Dex: 16, Con: 16, Int: 16, Wis: 16, Cha: 16,
				HPMax: 20, RealizedLevel: 3,
			})
			require.NoError(t, err)
			for _, code := range codes {
				_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: code, Equip: true})
				require.NoError(t, err)
			}
			warnings, err := bs.Items.Warnings(ctx, id, nil)
			require.NoError(t, err)
			out["warnings "+class+" "+name] = warnings
		}
		for _, race := range []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"} {
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Golden", Class: class, Race: race, Str: 16, Dex: 16, Con: 16, Int: 16, Wis: 16, Cha: 16,
				HPMax: 20, RealizedLevel: 3,
			})
			require.NoError(t, err)
			got, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			out["saves "+class+" "+race] = map[string]any{"saves": got.Derived.Saves, "raceSaveBonus": got.Derived.RaceSaveBonus}
		}
	}
	body, err := json.Marshal(out)
	require.NoError(t, err)
	golden(t, "races_equipment.json", body)
}

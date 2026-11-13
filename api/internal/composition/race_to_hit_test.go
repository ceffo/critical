package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0030 step 22.3 wired end to end: the fixture catalogue's race traits reach the attack
// preview through the derived block and the character view. An elf holding the catalogue's long
// sword is previewed with a race modifier of +1 (Detail the race code); with the light
// crossbow, none.
func TestAttackPreview_Step22_3_TheCatalogueTraitReachesTheAttack(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	for _, tc := range []struct {
		weapon string
		want   int
	}{
		{"long_sword", 1},
		{"light_crossbow", 0},
	} {
		t.Run(tc.weapon, func(t *testing.T) {
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Elf", Class: "fighter", Race: "elf", Str: 12, Dex: 12, Con: 12, Int: 12, Wis: 12, Cha: 12,
				HPMax: 10, RealizedLevel: 1,
			})
			require.NoError(t, err)
			_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: tc.weapon, Equip: true})
			require.NoError(t, err)

			form, err := bs.Combat.GetAttackForm(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, form.Preview)
			var race []combatsvc.Modifier
			for _, mod := range form.Preview.Modifiers {
				if mod.Kind == combatsvc.ModRace {
					race = append(race, mod)
				}
			}
			if tc.want == 0 {
				assert.Empty(t, race)
				return
			}
			assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModRace, Detail: "elf", Value: tc.want}}, race)
		})
	}
}

package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0030 step 22.3 wired end to end on the fixture catalogue's weapons: the race traits reach
// the attack preview and the roll through the derived block and the character view. The preview
// and the roll list the same modifiers, the race's right after Dexterity (ADR 0002 steps 2 and
// 12). The halfling's bonus counts with both slings (the staff sling is its own type) and with
// any weapon hurled at the target (a missile piece without ammunition: the player sets a picked
// spear's or club's kind to missile to throw it); the blowgun, and the same weapons in melee, get
// none.
func TestAttack_Step22_3_TheCatalogueTraitReachesTheAttack(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	homebrewThrownDagger := itemsvc.ItemInput{
		Name: localized.Of("Throwing dagger"), Category: itemsvc.CategoryWeapon, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		WeaponType: itemsvc.TypeDagger, WeaponKind: itemsvc.WeaponKindMissile, Size: "s", Speed: 2, DamageSM: "1d4", DamageL: "1d3",
	}
	hurl := &itemsvc.ItemPatch{WeaponKind: new(itemsvc.WeaponKindMissile)}
	wield := &itemsvc.ItemPatch{WeaponKind: new(itemsvc.WeaponKindMelee)}
	needles := &itemsvc.ItemPatch{Fires: new("needle")}
	for _, tc := range []struct {
		name, race string
		weapon     itemsvc.ItemInput
		edit       *itemsvc.ItemPatch // the player's edit after the pick, nil for none
		ammo       string             // catalogue code of the ammunition, "" for none
		want       int
	}{
		{"elf, long sword", "elf", itemsvc.ItemInput{CatalogueCode: "long_sword"}, nil, "", 1},
		{"elf, composite short bow and arrows", "elf", itemsvc.ItemInput{CatalogueCode: "composite_short_bow"}, nil, "flight_arrow", 1},
		{"elf, light crossbow", "elf", itemsvc.ItemInput{CatalogueCode: "light_crossbow"}, nil, "light_quarrel", 0},
		{"elf, thrown spear", "elf", itemsvc.ItemInput{CatalogueCode: "spear"}, hurl, "", 0},
		{"halfling, sling", "halfling", itemsvc.ItemInput{CatalogueCode: "sling"}, nil, "sling_bullet", 1},
		{"halfling, staff sling", "halfling", itemsvc.ItemInput{CatalogueCode: "staff_sling"}, nil, "sling_bullet", 1},
		{"halfling, blowgun", "halfling", itemsvc.ItemInput{CatalogueCode: "blowgun"}, needles, "needle", 0},
		{"halfling, thrown spear", "halfling", itemsvc.ItemInput{CatalogueCode: "spear"}, hurl, "", 1},
		{"halfling, spear in melee", "halfling", itemsvc.ItemInput{CatalogueCode: "spear"}, nil, "", 0},
		{"halfling, thrown hand axe", "halfling", itemsvc.ItemInput{CatalogueCode: "hand_axe"}, nil, "", 1},
		{"halfling, hand axe in melee", "halfling", itemsvc.ItemInput{CatalogueCode: "hand_axe"}, wield, "", 0},
		{"halfling, thrown club", "halfling", itemsvc.ItemInput{CatalogueCode: "club"}, hurl, "", 1},
		{"halfling, club in melee", "halfling", itemsvc.ItemInput{CatalogueCode: "club"}, nil, "", 0},
		{"halfling, homebrew missile dagger", "halfling", homebrewThrownDagger, nil, "", 1},
		{"halfling, catalogue dagger (melee)", "halfling", itemsvc.ItemInput{CatalogueCode: "dagger"}, nil, "", 0},
		{"halfling, short sword", "halfling", itemsvc.ItemInput{CatalogueCode: "short_sword"}, nil, "", 0},
		{"human, long sword", "human", itemsvc.ItemInput{CatalogueCode: "long_sword"}, nil, "", 0},
		{"human, thrown spear", "human", itemsvc.ItemInput{CatalogueCode: "spear"}, hurl, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Archer", Class: "fighter", Race: tc.race, Str: 12, Dex: 16, Con: 12, Int: 12, Wis: 12, Cha: 12,
				HPMax: 10, RealizedLevel: 1,
			})
			require.NoError(t, err)
			w := tc.weapon
			w.Equip = true
			piece, err := bs.Items.Create(ctx, id, w)
			require.NoError(t, err)
			if tc.edit != nil {
				_, err = bs.Items.Update(ctx, id, piece.Item.ID, *tc.edit)
				require.NoError(t, err)
			}
			if tc.ammo != "" {
				_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: tc.ammo, Count: 10})
				require.NoError(t, err)
			}

			form, err := bs.Combat.GetAttackForm(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, form.Preview)
			res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
				WeaponID: form.Form.WeaponID, AmmoItemID: form.Form.AmmoItemID, TargetAC: new(5),
			})
			require.NoError(t, err)
			assert.Equal(t, form.Preview.Modifiers, res.Modifiers, "the roll's breakdown is the preview's")

			var race []combatsvc.Modifier
			at, dex := -1, -1
			for i, mod := range res.Modifiers {
				switch mod.Kind {
				case combatsvc.ModRace:
					race, at = append(race, mod), i
				case combatsvc.ModDexterity:
					dex = i
				}
			}
			if tc.want == 0 {
				assert.Empty(t, race)
				return
			}
			assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModRace, Detail: tc.race, Value: tc.want}}, race)
			if dex >= 0 {
				assert.Equal(t, dex+1, at, "the race's bonus comes right after Dexterity")
			}
		})
	}
}

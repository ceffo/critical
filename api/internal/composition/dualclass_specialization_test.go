package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0042 step 12 (amended 2026-11-13, PHB p.44-45 and p.52) end to end: an elf who was a
// single-class fighter, left at 11 with a long sword specialization, now a mage, keeps the
// specialization once the mage is above 11: the sheet's specializes, the sword's specialized flag,
// the attack's +1 to hit and +2 damage as specialization modifiers and Table 35's specialist rate
// read at the fighter's 11 (2/1 at 7 to 12, where Table 15 gives 3/2 and Table 35 at the mage's
// 14 gives 5/2). At mage 11 (not above the fighter's 11) none of it applies: the fighter's rate
// is dormant, so no rate shows, as before the crossover for any old-class figure (step 13).
func TestResolveAttack_Step12_ADualClassFighterKeepsItsSpecializationPastTheCrossover(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mage, mageXP int
		specializes  bool
		specMods     []combatsvc.Modifier
		damage       int // the preview's damage bonus over Strength 17's +1
		rate         *combatsvc.AttackRateView
	}{
		{
			name: "mage 14, past the crossover", mage: 14, mageXP: 1_500_000, specializes: true,
			specMods: []combatsvc.Modifier{{Kind: combatsvc.ModSpecialization, Value: 1}}, damage: 2,
			rate: &combatsvc.AttackRateView{Attacks: 2, Rounds: 1},
		},
		{name: "mage 11, not above the fighter's 11", mage: 11, mageXP: 375_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			m := memory.New()
			bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Elf", Race: "elf", Class: "mage", RealizedLevel: tc.mage, XPTotal: tc.mageXP,
				Str: 17, Dex: 16, Con: 15, Int: 17, Wis: 10, Cha: 10, HPMax: 60,
				Classes: []classes.Entry{
					{Class: "fighter", RealizedLevel: 11, StartingXP: 750_000, State: classes.EntryFrozen, LeftAtLevel: 11},
					classes.SingleEntry("mage", tc.mage, tc.mageXP),
				},
				Proficiencies: skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{{Type: "long_sword", Slots: 2}}},
			})
			require.NoError(t, err)
			sword, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
			require.NoError(t, err)

			c, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tc.specializes, c.Derived.Specializes, "the sheet's specializes")
			items, err := bs.Items.List(ctx, id)
			require.NoError(t, err)
			for _, it := range items {
				if it.ID == sword.Item.ID {
					assert.Equal(t, tc.specializes, it.Specialized, "the sword's specialized flag")
				}
			}

			form, err := bs.Combat.GetAttackForm(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, form.Preview)
			var spec []combatsvc.Modifier
			for _, m := range form.Preview.Modifiers {
				if m.Kind == combatsvc.ModSpecialization {
					spec = append(spec, m)
				}
			}
			assert.Equal(t, tc.specMods, spec, "to hit")
			assert.Equal(t, 1+tc.damage, form.Preview.DamageBonus, "Strength 17's +1, +2 specialized")
			if form.AttackRate != nil {
				form.AttackRate.ThisRound = 0
			}
			assert.Equal(t, tc.rate, form.AttackRate, "Table 35 at the fighter's 11")
		})
	}
}

package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
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
				Proficiencies: skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: 2}}},
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

// ADR 0037 step 9 with ADR 0042 step 12 through the real services: the weapon editor offers a new
// specialization by the weapon types' specializationSlots (the web's lead and per-type costs read
// them), which only an active class that specializes gives; the character's specializes follows
// the kept one too. Per case: the derived specializes, the long sword's cost on the sheet, and the
// save of a long sword specialization (kept or new): refused not_allowed to a class that may not.
func TestSetProficiencies_Step12_TheSpecializationOfferFollowsTheActiveClass(t *testing.T) {
	spec := skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: 2}}}
	plain := skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: 1}}}
	for _, tc := range []struct {
		name        string
		c           charactersvc.Character
		specializes bool
		cost        int  // the long sword's specializationSlots on the sheet
		refused     bool // the save of the specialization
	}{
		{
			name: "elf fighter 11 then mage 14, the pick kept", specializes: true, cost: 0,
			c: charactersvc.Character{
				Race: "elf", Class: "mage", RealizedLevel: 14, XPTotal: 1_500_000, Proficiencies: spec,
				Classes: []classes.Entry{
					{Class: "fighter", RealizedLevel: 11, StartingXP: 750_000, State: classes.EntryFrozen, LeftAtLevel: 11},
					classes.SingleEntry("mage", 14, 1_500_000),
				},
			},
		},
		{
			name: "elf fighter 11 then mage 11, the pick kept", specializes: false, cost: 0,
			c: charactersvc.Character{
				Race: "elf", Class: "mage", RealizedLevel: 11, XPTotal: 375_000, Proficiencies: spec,
				Classes: []classes.Entry{
					{Class: "fighter", RealizedLevel: 11, StartingXP: 750_000, State: classes.EntryFrozen, LeftAtLevel: 11},
					classes.SingleEntry("mage", 11, 375_000),
				},
			},
		},
		{
			name: "single-class fighter 5", specializes: true, cost: 2,
			c: charactersvc.Character{Race: "human", Class: "fighter", RealizedLevel: 5, XPTotal: 16_000, Proficiencies: plain},
		},
		{
			name: "fighter/thief", specializes: false, cost: 0, refused: true,
			c: charactersvc.Character{
				Race: "half_elf", Class: "fighter", RealizedLevel: 5, XPTotal: 20_000, Proficiencies: plain,
				Classes: []classes.Entry{classes.SingleEntry("fighter", 5, 20_000), classes.SingleEntry("thief", 6, 20_000)},
			},
		},
		{
			name: "mage 5 then fighter 3", specializes: true, cost: 2,
			c: charactersvc.Character{
				Race: "human", Class: "fighter", RealizedLevel: 3, XPTotal: 5_000, Proficiencies: plain,
				Classes: []classes.Entry{
					{Class: "mage", RealizedLevel: 5, StartingXP: 40_000, State: classes.EntryFrozen, LeftAtLevel: 5},
					classes.SingleEntry("fighter", 3, 5_000),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			m := memory.New()
			bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
			c := tc.c
			c.Name, c.Str, c.Dex, c.Con, c.Int, c.Wis, c.Cha, c.HPMax = "Hero", 17, 16, 15, 17, 10, 10, 40
			id, err := m.AddCharacter(ctx, c)
			require.NoError(t, err)

			got, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tc.specializes, got.Derived.Specializes, "specializes")
			require.NotNil(t, got.Derived.Proficiencies)
			cost := -1
			for _, o := range got.Derived.Proficiencies.Weapons {
				if o.Code == "long_sword" {
					cost = o.SpecializationSlots
				}
			}
			assert.Equal(t, tc.cost, cost, "the long sword's specialization cost")

			_, err = bs.Skills.SetProficiencies(ctx, id, skillsvc.Edit{Picks: spec})
			if !tc.refused {
				require.NoError(t, err, "the specialization saves")
				return
			}
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: "not_allowed"}, *fe)
		})
	}
}

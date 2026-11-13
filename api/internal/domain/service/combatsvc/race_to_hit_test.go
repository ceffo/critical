package combatsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// The race's to-hit bonus (ADR 0030 step 22.3, the weapon_to_hit trait of ADR 0040 step 4): the
// traits are read from the fixture catalogue's race definitions, so a wrong or missing trait in
// the data fails here as well as a wrong rule.

var (
	raceLongSword = combatsvc.Weapon{
		Wielded: true, ID: "long_sword", Name: localized.Of("Long Sword"), BaseDamage: "1d8", DamageLarge: "1d12",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeLongSword,
	}
	raceShortSword = combatsvc.Weapon{
		Wielded: true, ID: "short_sword", Name: localized.Of("Short Sword"), BaseDamage: "1d6", DamageLarge: "1d8",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeShortSword,
	}
	// A composite long bow is of the long_bow type (ADR 0030 step 23).
	raceCompositeLongBow = combatsvc.Weapon{
		Wielded: true, ID: "composite_long_bow", Name: localized.Of("Composite Long Bow"), BaseDamage: "1d6", DamageLarge: "1d6",
		Kind: combatsvc.WeaponMissile, AmmoKind: "arrow", TwoHanded: true, Type: itemsvc.TypeLongBow,
	}
	raceLightCrossbow = combatsvc.Weapon{
		Wielded: true, ID: "light_crossbow", Name: localized.Of("Light Crossbow"), BaseDamage: "1d4", DamageLarge: "1d4",
		Kind: combatsvc.WeaponMissile, AmmoKind: "quarrel", TwoHanded: true, Type: itemsvc.TypeLightCrossbow,
	}
	raceSling = combatsvc.Weapon{
		Wielded: true, ID: "sling", Name: localized.Of("Sling"), BaseDamage: "1d4", DamageLarge: "1d4",
		Kind: combatsvc.WeaponMissile, AmmoKind: "bullet", Type: itemsvc.TypeSling,
	}
	// A hurled dagger is a missile piece of the dagger type (ADR 0016 step 8).
	raceThrownDagger = combatsvc.Weapon{
		Wielded: true, ID: "throwing_dagger", Name: localized.Of("Throwing Dagger"), BaseDamage: "1d4", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMissile, Type: itemsvc.TypeDagger,
	}
	raceQuarrels = itemsvc.Item{ID: "quarrels", Name: localized.Of("Quarrels"), Category: itemsvc.CategoryAmmunition, AmmoKind: "quarrel", Count: 10}
	raceBullets  = itemsvc.Item{ID: "bullets", Name: localized.Of("Bullets"), Category: itemsvc.CategoryAmmunition, AmmoKind: "bullet", Count: 10}
)

// ammoFor is the ammunition the launcher fires in these tests, nil for a weapon without.
func ammoFor(w combatsvc.Weapon) *itemsvc.Item {
	switch w.AmmoKind {
	case "arrow":
		return &plainArrows
	case "quarrel":
		return &raceQuarrels
	case "bullet":
		return &raceBullets
	}
	return nil
}

// raceChar is a level 5 fighter of race (STR 12, DEX 17: +2 missile) proficient with the weapon
// type, so no penalty mixes in, with the race's weapon_to_hit trait as the fixture catalogue
// defines it (none for a race without the trait).
func raceChar(t *testing.T, race, weaponType string) charactersvc.View {
	t.Helper()
	v := profChar(t, "fighter", weaponType)
	v.Race = race
	r, ok := cataloguetest.Definitions(t).Race(race)
	require.True(t, ok, "no race %s in the fixture", race)
	if tr, ok := r.Trait(classes.TraitWeaponToHit); ok {
		v.RaceToHitTypes, v.RaceToHitBonus = tr.WeaponTypes, tr.Bonus
	}
	return v
}

type raceToHitCase struct {
	name, race string
	weapon     combatsvc.Weapon
	// bonus is the expected race modifier's value, 0 for none.
	bonus int
}

var raceToHitCases = []raceToHitCase{
	{"elf with a long sword: +1", "elf", raceLongSword, 1},
	{"elf with a short sword: +1", "elf", raceShortSword, 1},
	{"elf with a short bow: +1", "elf", plainShortBow, 1},
	{"elf with a composite long bow: +1", "elf", raceCompositeLongBow, 1},
	{"elf with a light crossbow: none", "elf", raceLightCrossbow, 0},
	{"elf with a dagger: none", "elf", plainDagger, 0},
	{"halfling with a sling: +1", "halfling", raceSling, 1},
	{"halfling with a hurled dagger: +1", "halfling", raceThrownDagger, 1},
	{"halfling with a dagger in melee: none (not thrown)", "halfling", plainDagger, 0},
	{"halfling with a short sword: none", "halfling", raceShortSword, 0},
	{"halfling with a short bow: none", "halfling", plainShortBow, 0},
	{"human with a long sword: none", "human", raceLongSword, 0},
	{"dwarf with a short bow: none", "dwarf", plainShortBow, 0},
}

// rollWith resolves one attack by char with w against AC 5, with fixed dice: d20 13, no crit,
// the damage die 3.
func rollWith(t *testing.T, char charactersvc.View, w combatsvc.Weapon) *combatsvc.RollResult {
	t.Helper()
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, w.ID).Return(w, nil)
	f.withChar(char)
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(strMods(12), nil)
	req := combatsvc.AttackRequest{WeaponID: w.ID, TargetAC: intPtr(5)}
	if a := ammoFor(w); a != nil {
		f.items.EXPECT().Use(mock.Anything, charID, a.ID).Return(*a, nil).Once()
		req.AmmoItemID = a.ID
	}
	f.dice.EXPECT().RollD20().Return(13).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(mock.Anything).Return(3).Once()
	f.records()
	res, err := f.svc.ResolveAttack(t.Context(), charID, req)
	require.NoError(t, err)
	return res
}

// withRace is the modifiers without a race bonus with the race's term put after the ability
// terms (Strength, Dexterity), where ADR 0030 step 22.3 lists it.
func withRace(base []combatsvc.Modifier, race string, bonus int) []combatsvc.Modifier {
	if bonus == 0 {
		return base
	}
	at := 0
	for at < len(base) && (base[at].Kind == combatsvc.ModStrength || base[at].Kind == combatsvc.ModDexterity) {
		at++
	}
	out := append([]combatsvc.Modifier{}, base[:at]...)
	out = append(out, combatsvc.Modifier{Kind: combatsvc.ModRace, Detail: race, Value: bonus})
	return append(out, base[at:]...)
}

// noRaceTrait is char without its race's to-hit trait: the same attack with no race bonus.
func noRaceTrait(char charactersvc.View) charactersvc.View {
	char.RaceToHitTypes, char.RaceToHitBonus = nil, 0
	return char
}

// ADR 0030 step 22.3: an elf gets +1 to hit with bows of any sort but crossbows and with short
// and long swords; a halfling with slings and thrown weapons (a dagger only when hurled), PHB
// chapter 2. The bonus is one named modifier of kind race (Detail the race code), read from the
// weapon's type, after the ability terms; it moves the effective THAC0 and nothing else.
func TestResolveAttack_Step22_3_RaceToHitByWeaponType(t *testing.T) {
	for _, tc := range raceToHitCases {
		t.Run(tc.name, func(t *testing.T) {
			char := raceChar(t, tc.race, tc.weapon.Type)
			res := rollWith(t, char, tc.weapon)
			base := rollWith(t, noRaceTrait(char), tc.weapon)

			assert.Equal(t, withRace(base.Modifiers, tc.race, tc.bonus), res.Modifiers)
			assert.Equal(t, base.EffectiveThac0-tc.bonus, res.EffectiveThac0)
		})
	}
}

// ADR 0030 step 22.3 (the user's ruling, 2026-11-13: to hit only): the race's bonus adds no
// damage term and leaves the damage as it is without it.
func TestResolveAttack_Step22_3_RaceBonusIsNoDamage(t *testing.T) {
	for _, tc := range raceToHitCases {
		if tc.bonus == 0 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			char := raceChar(t, tc.race, tc.weapon.Type)
			res := rollWith(t, char, tc.weapon)
			base := rollWith(t, noRaceTrait(char), tc.weapon)

			require.Contains(t, res.Modifiers, combatsvc.Modifier{Kind: combatsvc.ModRace, Detail: tc.race, Value: tc.bonus},
				"the race's bonus applies to hit")
			require.True(t, res.Hit, "the 13 hits")
			for _, d := range res.DamageTerms {
				assert.NotEqual(t, combatsvc.ModRace, d.Kind, "a race damage term")
			}
			assert.Equal(t, base.DamageTerms, res.DamageTerms)
			assert.Equal(t, base.TotalDamage, res.TotalDamage)
		})
	}
}

// ADR 0030 step 22.3 with ADR 0002 step 12: the attack preview lists the race's bonus exactly
// as the attack does, with the same effective THAC0, and no damage from it.
func TestPreviewAttack_Step22_3_ShowsTheRaceBonus(t *testing.T) {
	for _, tc := range raceToHitCases {
		t.Run(tc.name, func(t *testing.T) {
			char := raceChar(t, tc.race, tc.weapon.Type)
			res := rollWith(t, char, tc.weapon)

			preview := func(c charactersvc.View) *combatsvc.AttackPreview {
				f := newFixture(t)
				form := combatsvc.AttackForm{WeaponID: tc.weapon.ID, TargetAC: intPtr(5), TargetSize: combatsvc.SizeSmallMedium}
				items := []itemsvc.Item{}
				if a := ammoFor(tc.weapon); a != nil {
					form.AmmoItemID = a.ID
					items = append(items, *a)
				}
				f.forms.EXPECT().Get(mock.Anything, charID).Return(form, true, nil).Once()
				f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{tc.weapon}, nil)
				f.items.EXPECT().List(mock.Anything, charID).Return(items, nil).Maybe()
				f.withChar(c)
				f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(strMods(12), nil)
				f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(1, nil).Maybe()
				v, err := f.svc.GetAttackForm(t.Context(), charID)
				require.NoError(t, err)
				require.NotNil(t, v.Preview)
				return v.Preview
			}
			p, base := preview(char), preview(noRaceTrait(char))

			assert.Equal(t, res.Modifiers, p.Modifiers)
			assert.Equal(t, res.EffectiveThac0, p.EffectiveThac0)
			assert.Equal(t, tc.bonus, p.ToHit-base.ToHit)
			assert.Equal(t, base.DamageBonus, p.DamageBonus)
		})
	}
}

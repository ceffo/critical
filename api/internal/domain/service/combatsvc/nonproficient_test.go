package combatsvc_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// The non-proficiency penalty (ADR 0037 step 8, by the weapon's catalogue code since step 21.3):
// the penalties are read from the fixture's Table 34 (seeds/proficiencies.json) through
// skills.ProficiencyFigures, as the character service derives them, so a wrong seed row or a
// figure not carried fails here.

const profTHAC0 = 18

var (
	plainDagger = combatsvc.Weapon{
		Wielded: true, ID: "dagger", Name: localized.Of("Dagger"), BaseDamage: "1d4", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeDagger, Code: "dagger",
	}
	plainShortBow = combatsvc.Weapon{
		Wielded: true, ID: "short_bow", Name: localized.Of("Short Bow"), BaseDamage: "1d6", DamageLarge: "1d6",
		Kind: combatsvc.WeaponMissile, AmmoKind: "arrow", TwoHanded: true, Type: itemsvc.TypeShortBow, Code: "short_bow",
	}
	// homebrewClub has no catalogue code (a homebrew weapon), though it has a type.
	homebrewClub = combatsvc.Weapon{
		Wielded: true, ID: "club", Name: localized.Of("Club"), BaseDamage: "1d6", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeOtherMelee,
	}
	// heavyLance is the heavy horse lance: other_melee, as the quarterstaff is.
	heavyLance = combatsvc.Weapon{
		Wielded: true, ID: "lance", Name: localized.Of("Heavy Horse Lance"), BaseDamage: "1d8", DamageLarge: "3d6",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeOtherMelee, Code: "heavy_horse_lance",
	}
	plainArrows = itemsvc.Item{ID: "arrows", Name: localized.Of("Arrows"), Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Count: 12}
)

// profChar is a level 5 character of class with the weapon picks (catalogue codes), its proficiency figures
// derived from the fixture catalogue (STR 12: no Strength adjustment; DEX 17: +2 missile).
func profChar(t *testing.T, class string, picks ...string) charactersvc.View {
	t.Helper()
	book, err := ruletablesadpt.ReadProficiencies(filepath.Join("..", "..", "..", "..", "testdata", "seeds"), cataloguetest.Definitions(t))
	require.NoError(t, err)
	eq, ok := cataloguetest.Tables(t).Equipment()
	require.True(t, ok)
	book, err = book.WithWeapons(eq.PickWeapons())
	require.NoError(t, err)
	weapon := make([]skills.WeaponPick, 0, len(picks))
	for _, p := range picks {
		weapon = append(weapon, skills.WeaponPick{Code: p, Slots: 1})
	}
	f, ok := skills.ProficiencyFigures(book, class, 5, 12, skills.Spending{Picks: skills.Picks{Weapon: weapon}})
	require.True(t, ok, "no Table 34 row for %s", class)
	v := viewtest.Single(class, 5, viewtest.WithIdentity(charID, "Test", "human"), viewtest.WithScores(12, 17, 0, 0, 0, 0), viewtest.WithDexAdjust(2, -3))
	v.THAC0, v.Proficiencies = profTHAC0, &f
	return v
}

func nonprof(code string, value int) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModNonproficient, Detail: code, Value: value}
}

var str12Mod = combatsvc.Modifier{Kind: combatsvc.ModStrength, Detail: "12", Value: 0}

type nonprofCase struct {
	name   string
	char   func(t *testing.T) charactersvc.View
	weapon combatsvc.Weapon
	// penalty is the expected modifier's value, 0 for none.
	penalty int
}

var nonprofCases = []nonprofCase{
	{"warrior (fighter) without a pick: -2", func(t *testing.T) charactersvc.View { return profChar(t, "fighter") }, plainDagger, -2},
	{"wizard (mage) without a pick: -5", func(t *testing.T) charactersvc.View { return profChar(t, "mage") }, plainDagger, -5},
	{"priest (cleric) without a pick: -3", func(t *testing.T) charactersvc.View { return profChar(t, "cleric") }, plainDagger, -3},
	{"rogue (thief) without a pick: -3", func(t *testing.T) charactersvc.View { return profChar(t, "thief") }, plainDagger, -3},
	{"rogue (bard) without a pick: -3", func(t *testing.T) charactersvc.View { return profChar(t, "bard") }, plainDagger, -3},
	{"a pick of another weapon does not cover it", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", "long_sword")
	}, plainDagger, -2},
	{"fighter with the weapon picked: none", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", "dagger")
	}, plainDagger, 0},
	{"mage with the weapon picked: none", func(t *testing.T) charactersvc.View {
		return profChar(t, "mage", "dart", "dagger")
	}, plainDagger, 0},
	// A kept pick the class may not make (served allowed: false, step 7) is still a pick.
	{"cleric with a kept pick its class may not make: none", func(t *testing.T) charactersvc.View {
		c := profChar(t, "cleric", "dagger")
		require.False(t, c.Proficiencies.WeaponPicks[0].Allowed, "the cleric may not pick dagger")
		return c
	}, plainDagger, 0},
}

// ADR 0037 step 8 (by code since step 21.3): an attack with a weapon whose catalogue code has no
// pick takes Table 34's penalty for the class group, once, as the named modifier nonproficient
// (Detail the code); with a pick of the code, none. It moves the effective THAC0 and the roll
// needed.
func TestResolveAttack_Step8_NonproficientPenaltyByGroup(t *testing.T) {
	for _, tt := range nonprofCases {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, charID, tt.weapon.ID).Return(tt.weapon, nil)
			f.withChar(tt.char(t))
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(strMods(12), nil)
			f.dice.EXPECT().RollD20().Return(13).Once()
			f.d100s(t, 90, noCritMult)
			f.dice.EXPECT().RollDie(4).Return(3).Once()
			f.records()

			res, err := f.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{WeaponID: tt.weapon.ID, TargetAC: intPtr(5)})
			require.NoError(t, err)
			want := []combatsvc.Modifier{str12Mod}
			if tt.penalty != 0 {
				want = append(want, nonprof("dagger", tt.penalty))
			}
			assert.Equal(t, want, res.Modifiers)
			assert.Equal(t, profTHAC0-tt.penalty, res.EffectiveThac0)
			// THAC0 18 - AC 5 = 13 without a penalty: the 13 hits only then.
			assert.Equal(t, tt.penalty == 0, res.Hit)
			if tt.penalty != 0 {
				assert.Contains(t, string(f.entries[0].Payload), `"kind":"nonproficient"`)
			}
		})
	}
}

// ADR 0037 step 8 (by code since step 21.3): a launcher is judged by its own catalogue code,
// never by its ammunition, nor by another bow of its type (the composite short bow's pick does not
// cover the short bow); the penalty sits after the buffs and before the situational modifier
// (amends ADR 0002 steps 2 and 12).
func TestResolveAttack_Step8_LauncherByItsTypeAndOrder(t *testing.T) {
	bless := buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: 1,
		ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}},
	}
	for _, tc := range []struct {
		name    string
		picks   []string
		penalty int
	}{
		{"short bow picked", []string{"short_bow"}, 0},
		{"long bow picked, not the short one", []string{"long_bow"}, -3},
		{"composite short bow picked, not the short bow", []string{"composite_short_bow"}, -3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, charID, plainShortBow.ID).Return(plainShortBow, nil)
			f.withChar(profChar(t, "thief", tc.picks...))
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("missile")).Return(bless, nil)
			f.items.EXPECT().Use(mock.Anything, charID, plainArrows.ID).Return(plainArrows, nil).Once()
			f.dice.EXPECT().RollD20().Return(13).Once()
			f.d100s(t, 90, noCritMult)
			f.dice.EXPECT().RollDie(6).Return(4).Once()
			f.records()

			res, err := f.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{
				WeaponID: plainShortBow.ID, AmmoItemID: plainArrows.ID, TargetAC: intPtr(5),
				Situational: &combatsvc.Situational{Value: -1, Label: "cover"},
			})
			require.NoError(t, err)
			want := []combatsvc.Modifier{
				{Kind: combatsvc.ModDexterity, Detail: "17", Value: 2},
				{Kind: combatsvc.ModBuff, Detail: "Bless", Value: 1},
			}
			if tc.penalty != 0 {
				want = append(want, nonprof("short_bow", tc.penalty))
			}
			want = append(want, combatsvc.Modifier{Kind: combatsvc.ModSituational, Detail: "cover", Value: -1})
			assert.Equal(t, want, res.Modifiers)
			assert.Equal(t, profTHAC0-2-tc.penalty, res.EffectiveThac0)
		})
	}
}

// codeCases are ADR 0037 steps 21.3 and 21.4: a fighter's quarterstaff pick no longer covers
// the heavy horse lance (both other_melee, one type pick covered both before); the lance's own
// pick does; a weapon with no catalogue code is non-proficient whatever the picks.
var codeCases = []nonprofCase{
	{"a quarterstaff pick, attacking with the heavy horse lance: -2", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", "quarterstaff")
	}, heavyLance, -2},
	{"the heavy horse lance picked: none", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", "quarterstaff", "heavy_horse_lance")
	}, heavyLance, 0},
	{"a homebrew club with no code, any picks: -5", func(t *testing.T) charactersvc.View {
		return profChar(t, "mage", "quarterstaff", "dagger")
	}, homebrewClub, -5},
}

// attackWith resolves an attack of tc's character with tc's weapon (a d20 of 13 against AC 5).
func attackWith(t *testing.T, tc nonprofCase) *combatsvc.RollResult {
	t.Helper()
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, tc.weapon.ID).Return(tc.weapon, nil)
	f.withChar(tc.char(t))
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(strMods(12), nil)
	f.dice.EXPECT().RollD20().Return(13).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(sidesOf(tc.weapon.BaseDamage)).Return(3).Once()
	f.records()
	res, err := f.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{WeaponID: tc.weapon.ID, TargetAC: intPtr(5)})
	require.NoError(t, err)
	return res
}

// sidesOf is the die of a one-die damage formula ("1d8": 8).
func sidesOf(formula string) int {
	n := 0
	for _, r := range formula[2:] {
		n = n*10 + int(r-'0')
	}
	return n
}

// ADR 0037 step 21.3: the penalty applies when no pick names the weapon's catalogue code: a
// fighter with a quarterstaff pick attacking with a heavy horse lance takes -2, with the lance's
// pick none; the modifier's detail is the code.
func TestResolveAttack_Step21_3_PenaltyByTheWeaponsCode(t *testing.T) {
	for _, tc := range codeCases[:2] {
		t.Run(tc.name, func(t *testing.T) {
			res := attackWith(t, tc)
			want := []combatsvc.Modifier{str12Mod}
			if tc.penalty != 0 {
				want = append(want, nonprof("heavy_horse_lance", tc.penalty))
			}
			assert.Equal(t, want, res.Modifiers)
			assert.Equal(t, profTHAC0-tc.penalty, res.EffectiveThac0)
		})
	}
}

// ADR 0037 step 21.4 (the proposed reading): a weapon piece with no catalogue code is
// non-proficient: it takes the penalty whatever the picks, its detail empty; the piece carries
// weapon_code_missing (itemsvc, TestClassWarnings_Step21_4_WeaponCodeMissing).
func TestResolveAttack_Step21_4_NoCodeIsNonproficient(t *testing.T) {
	res := attackWith(t, codeCases[2])
	assert.Equal(t, []combatsvc.Modifier{str12Mod, nonprof("", -5)}, res.Modifiers)
	assert.Equal(t, profTHAC0+5, res.EffectiveThac0)
	assert.False(t, res.Hit)
}

// ADR 0037 step 8 with ADR 0002 step 12: the preview lists the penalty exactly as the attack
// does, for every case of TestResolveAttack_Step8_NonproficientPenaltyByGroup, rolling nothing.
func TestPreviewAttack_Step8_ShowsThePenalty(t *testing.T) {
	previewMatchesTheAttack(t, nonprofCases)
}

// ADR 0037 step 21.3 with ADR 0002 step 12: the preview shows the penalty by code exactly as the
// attack does (TestResolveAttack_Step21_3_PenaltyByTheWeaponsCode and
// TestResolveAttack_Step21_4_NoCodeIsNonproficient), rolling nothing.
func TestPreviewAttack_Step21_3_ShowsThePenaltyByCode(t *testing.T) {
	previewMatchesTheAttack(t, codeCases)
}

// previewMatchesTheAttack checks, for each case, that the attack form's preview lists the
// modifiers, THAC0 and roll needed the attack resolves.
func previewMatchesTheAttack(t *testing.T, cases []nonprofCase) {
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			char := tt.char(t)

			roll := newFixture(t)
			roll.weapons.EXPECT().Get(mock.Anything, charID, tt.weapon.ID).Return(tt.weapon, nil)
			roll.withChar(char)
			roll.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(strMods(12), nil)
			roll.dice.EXPECT().RollD20().Return(13).Once()
			roll.d100s(t, 90, noCritMult)
			roll.dice.EXPECT().RollDie(sidesOf(tt.weapon.BaseDamage)).Return(3).Once()
			roll.records()
			res, err := roll.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{WeaponID: tt.weapon.ID, TargetAC: intPtr(5)})
			require.NoError(t, err)

			f := newFixture(t)
			f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{
				WeaponID: tt.weapon.ID, TargetAC: intPtr(5), TargetSize: combatsvc.SizeSmallMedium,
			}, true, nil).Once()
			f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{tt.weapon}, nil)
			f.withChar(char)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(strMods(12), nil)
			f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(1, nil).Maybe()

			v, err := f.svc.GetAttackForm(t.Context(), charID)
			require.NoError(t, err)
			require.NotNil(t, v.Preview)
			assert.Equal(t, res.Modifiers, v.Preview.Modifiers)
			assert.Equal(t, res.EffectiveThac0, v.Preview.EffectiveThac0)
			assert.Equal(t, profTHAC0-res.EffectiveThac0, v.Preview.ToHit)
			require.NotNil(t, v.Preview.RollNeeded)
			assert.Equal(t, res.EffectiveThac0-5, *v.Preview.RollNeeded)
		})
	}
}

// ADR 0037 step 8: a spell's to-hit roll takes no non-proficiency penalty, even for a
// character with no weapon pick at all.
func TestRollSpellToHit_Step8_NoPenalty(t *testing.T) {
	f := newFixture(t)
	f.withChar(profChar(t, "mage"))
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponNone).Return(strMods(12), nil)
	f.dice.EXPECT().RollD20().Return(9).Once()

	res, err := f.svc.RollSpellToHit(t.Context(), charID, 5)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{str12Mod, {Kind: combatsvc.ModDexterity, Detail: "17", Value: 2}}, res.Modifiers)
	assert.Equal(t, profTHAC0-2, res.EffectiveThac0)
}

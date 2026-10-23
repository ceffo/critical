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

// The non-proficiency penalty (ADR 0037 step 8): the penalties are read from the fixture's
// Table 34 (seeds/proficiencies.json) through skills.ProficiencyFigures, as the
// character service derives them, so a wrong seed row or a figure not carried fails here.

const profTHAC0 = 18

var (
	plainDagger = combatsvc.Weapon{
		Wielded: true, ID: "dagger", Name: localized.Of("Dagger"), BaseDamage: "1d4", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMelee, Type: itemsvc.TypeDagger,
	}
	plainShortBow = combatsvc.Weapon{
		Wielded: true, ID: "short_bow", Name: localized.Of("Short Bow"), BaseDamage: "1d6", DamageLarge: "1d6",
		Kind: combatsvc.WeaponMissile, AmmoKind: "arrow", TwoHanded: true, Type: itemsvc.TypeShortBow,
	}
	// homebrewClub has no weapon type (a homebrew weapon never given one).
	homebrewClub = combatsvc.Weapon{
		Wielded: true, ID: "club", Name: localized.Of("Club"), BaseDamage: "1d6", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMelee,
	}
	plainArrows = itemsvc.Item{ID: "arrows", Name: localized.Of("Arrows"), Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Count: 12}
)

// profChar is a level 5 character of class with the weapon picks, its proficiency figures
// derived from the fixture catalogue (STR 12: no Strength adjustment; DEX 17: +2 missile).
func profChar(t *testing.T, class string, picks ...string) charactersvc.View {
	t.Helper()
	book, err := ruletablesadpt.ReadProficiencies(filepath.Join("..", "..", "..", "..", "testdata", "seeds"), cataloguetest.Definitions(t))
	require.NoError(t, err)
	weapon := make([]skills.WeaponPick, 0, len(picks))
	for _, p := range picks {
		weapon = append(weapon, skills.WeaponPick{Type: p, Slots: 1})
	}
	f, ok := skills.ProficiencyFigures(book, class, 5, 12, skills.Spending{Picks: skills.Picks{Weapon: weapon}})
	require.True(t, ok, "no Table 34 row for %s", class)
	v := viewtest.Single(class, 5, viewtest.WithIdentity(charID, "Test", "human"), viewtest.WithScores(12, 17, 0, 0, 0, 0))
	v.THAC0, v.Proficiencies = profTHAC0, &f
	return v
}

func nonprof(weaponType string, value int) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModNonproficient, Detail: weaponType, Value: value}
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
	{"a pick of another type does not cover it", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", itemsvc.TypeLongSword)
	}, plainDagger, -2},
	{"fighter with the type picked: none", func(t *testing.T) charactersvc.View {
		return profChar(t, "fighter", itemsvc.TypeDagger)
	}, plainDagger, 0},
	{"mage with the type picked: none", func(t *testing.T) charactersvc.View {
		return profChar(t, "mage", itemsvc.TypeDart, itemsvc.TypeDagger)
	}, plainDagger, 0},
	// A kept pick the class may not make (served allowed: false, step 7) is still a pick.
	{"cleric with a kept pick its class may not make: none", func(t *testing.T) charactersvc.View {
		c := profChar(t, "cleric", itemsvc.TypeDagger)
		require.False(t, c.Proficiencies.WeaponPicks[0].Allowed, "the cleric may not pick dagger")
		return c
	}, plainDagger, 0},
}

// ADR 0037 step 8: an attack with a weapon whose type has no pick takes Table 34's penalty
// for the class group, once, as the named modifier nonproficient (Detail the weapon type);
// with a pick of the type, none. It moves the effective THAC0 and the roll needed.
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
				want = append(want, nonprof(itemsvc.TypeDagger, tt.penalty))
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

// ADR 0037 step 8: a launcher is judged by its own weapon type, never by its ammunition; the
// penalty sits after the buffs and before the situational modifier (amends ADR 0002 steps 2
// and 12).
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
		{"short bow picked", []string{itemsvc.TypeShortBow}, 0},
		{"long bow picked, not the short one", []string{itemsvc.TypeLongBow}, -3},
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
				want = append(want, nonprof(itemsvc.TypeShortBow, tc.penalty))
			}
			want = append(want, combatsvc.Modifier{Kind: combatsvc.ModSituational, Detail: "cover", Value: -1})
			assert.Equal(t, want, res.Modifiers)
			assert.Equal(t, profTHAC0-2-tc.penalty, res.EffectiveThac0)
		})
	}
}

// ADR 0037 step 8 (proposed there): a weapon with no weapon type takes no penalty, whatever the
// picks; the piece carries the weapon_type_missing warning instead (itemsvc,
// TestClassWarnings_Step8_WeaponTypeMissing).
func TestResolveAttack_Step8_NoTypeNoPenalty(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, homebrewClub.ID).Return(homebrewClub, nil)
	f.withChar(profChar(t, "mage"))
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(strMods(12), nil)
	f.dice.EXPECT().RollD20().Return(13).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(6).Return(2).Once()
	f.records()

	res, err := f.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{WeaponID: homebrewClub.ID, TargetAC: intPtr(5)})
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{str12Mod}, res.Modifiers)
	assert.Equal(t, profTHAC0, res.EffectiveThac0)
}

// ADR 0037 step 8 with ADR 0002 step 12: the preview lists the penalty exactly as the attack
// does, for every case of TestResolveAttack_Step8_NonproficientPenaltyByGroup, rolling nothing.
func TestPreviewAttack_Step8_ShowsThePenalty(t *testing.T) {
	for _, tt := range nonprofCases {
		t.Run(tt.name, func(t *testing.T) {
			char := tt.char(t)

			roll := newFixture(t)
			roll.weapons.EXPECT().Get(mock.Anything, charID, tt.weapon.ID).Return(tt.weapon, nil)
			roll.withChar(char)
			roll.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(strMods(12), nil)
			roll.dice.EXPECT().RollD20().Return(13).Once()
			roll.d100s(t, 90, noCritMult)
			roll.dice.EXPECT().RollDie(4).Return(3).Once()
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

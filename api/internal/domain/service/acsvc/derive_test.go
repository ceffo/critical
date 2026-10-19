package acsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

func ptr[T any](v T) *T { return &v }

func armour(id string, base int, state itemsvc.State) itemsvc.Item {
	return itemsvc.Item{ID: id, Category: itemsvc.CategoryArmour, BaseAC: base, State: state}
}

func shield(id string, melee, missile int) itemsvc.Item {
	return itemsvc.Item{ID: id, Category: itemsvc.CategoryShield, ACMelee: melee, ACMissile: missile, State: itemsvc.StateWorn}
}

func ring(id, code string) itemsvc.Item {
	return itemsvc.Item{ID: id, Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicRing, CatalogueCode: code, State: itemsvc.StateWorn}
}

func acPower(itemID string, ac int) buffsvc.ItemPower {
	return buffsvc.ItemPower{ItemID: itemID, PowerID: "magic", Effects: buffsvc.BuffEffects{AC: ptr(ac)}}
}

func acBuff(name string, ac int, applies bool) buffsvc.BuffStatus {
	return buffsvc.BuffStatus{BuffEffect: buffsvc.BuffEffect{Name: name, Effects: buffsvc.BuffEffects{AC: ptr(ac)}}, Applies: applies}
}

func baseBuff(name, spellID string, base int) buffsvc.BuffStatus {
	return buffsvc.BuffStatus{BuffEffect: buffsvc.BuffEffect{Name: name, SpellID: spellID, Effects: buffsvc.BuffEffects{BaseAC: ptr(base)}}, Applies: true}
}

// rowsOf keeps the rows of term t.
func rowsOf(ac acsvc.AC, t acsvc.Term) []acsvc.Row {
	var out []acsvc.Row
	for _, r := range ac.Rows {
		if r.Term == t {
			out = append(out, r)
		}
	}
	return out
}

// ADR 0033 step 10, every term in its order, on the design round's example
// (ADR 0035 item 8): chain mail worn (AC 5, no magic), no shield, Dexterity
// 17 (-3), a ring of protection +2 worn (-2) and Protection from Evil (-2):
// AC -2. Each row names its source; a term with nothing is one empty row.
func TestAC_Step10_TermsInOrderWithTheirSources(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{
		Items: []itemsvc.Item{
			armour("chain", 5, itemsvc.StateWorn),
			ring("ring", ""),
		},
		Buffs:     []buffsvc.BuffStatus{{BuffEffect: buffsvc.BuffEffect{Name: "Protection from Evil", SpellID: "protection_from_evil", Effects: buffsvc.BuffEffects{AC: ptr(-2)}}, Applies: true}},
		Powers:    []buffsvc.ItemPower{acPower("ring", -2)},
		Dexterity: 17, DexAdjust: -3,
	})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermBase, ItemID: "chain", Value: 5},
		{Term: acsvc.TermArmourMagic},
		{Term: acsvc.TermShield},
		{Term: acsvc.TermDexterity, Value: -3, Dexterity: 17},
		{Term: acsvc.TermProtection, ItemID: "ring", Value: -2},
		{Term: acsvc.TermBuffs, Buff: "Protection from Evil", SpellID: "protection_from_evil", Value: -2},
	}, ac.Rows)
	assert.Equal(t, -2, ac.Value)
	assert.Nil(t, ac.Missile, "no body shield: one AC")
	assert.False(t, ac.Floored)
}

// ADR 0033 step 10.1: the worn body armour's base AC; with none worn the
// best base_ac effect that applies (step 11), else 10. A carried armour is
// not worn.
func TestAC_Step10_1_Base(t *testing.T) {
	cases := []struct {
		name  string
		in    acsvc.Inputs
		value int
		rows  []acsvc.Row
	}{
		{
			"no armour, no effect: 10",
			acsvc.Inputs{},
			10,
			[]acsvc.Row{{Term: acsvc.TermBase, Value: 10}},
		},
		{
			"a carried armour is not worn",
			acsvc.Inputs{Items: []itemsvc.Item{armour("plate", 3, itemsvc.StateCarried)}},
			10,
			[]acsvc.Row{{Term: acsvc.TermBase, Value: 10}},
		},
		{
			"the worn armour",
			acsvc.Inputs{Items: []itemsvc.Item{armour("plate", 3, itemsvc.StateWorn)}},
			3,
			[]acsvc.Row{{Term: acsvc.TermBase, ItemID: "plate", Value: 3}},
		},
		{
			"the Armor spell with no armour",
			acsvc.Inputs{Buffs: []buffsvc.BuffStatus{baseBuff("Armor", "armor", 6)}},
			6,
			[]acsvc.Row{{Term: acsvc.TermBase, Buff: "Armor", SpellID: "armor", Value: 6}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ac := acsvc.Derive(c.in)
			assert.Equal(t, c.rows, rowsOf(ac, acsvc.TermBase))
			assert.Equal(t, c.value, ac.Value)
		})
	}
}

// ADR 0033 step 11: base_ac replaces the base 10 only while no body armour
// is worn, the best (lowest) one counts and Dexterity still adds; the others
// are shown as not adding. A buff that does not apply gives none.
func TestAC_Step11_BaseACOnlyWithoutArmourTheBestCounts(t *testing.T) {
	bracers := itemsvc.Item{ID: "bracers", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicMisc, State: itemsvc.StateWorn}
	bracersAC4 := buffsvc.ItemPower{ItemID: "bracers", PowerID: "magic", Effects: buffsvc.BuffEffects{BaseAC: ptr(4)}}

	ac := acsvc.Derive(acsvc.Inputs{
		Items:     []itemsvc.Item{bracers},
		Buffs:     []buffsvc.BuffStatus{baseBuff("Armor", "armor", 6)},
		Powers:    []buffsvc.ItemPower{bracersAC4},
		Dexterity: 16, DexAdjust: -2,
	})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermBase, ItemID: "bracers", Value: 4},
		{Term: acsvc.TermBase, Buff: "Armor", SpellID: "armor", Value: 6, Off: acsvc.OffNotBest},
	}, rowsOf(ac, acsvc.TermBase))
	assert.Equal(t, 2, ac.Value, "bracers AC 4, Dexterity -2")

	worn := acsvc.Derive(acsvc.Inputs{
		Items:  []itemsvc.Item{armour("leather", 8, itemsvc.StateWorn), bracers},
		Powers: []buffsvc.ItemPower{bracersAC4},
	})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermBase, ItemID: "leather", Value: 8},
		{Term: acsvc.TermBase, ItemID: "bracers", Value: 4, Off: acsvc.OffArmourWorn},
	}, rowsOf(worn, acsvc.TermBase))
	assert.Equal(t, 8, worn.Value, "with armour worn, bracers give nothing")

	off := baseBuff("Armor", "armor", 6)
	off.Applies = false
	assert.Equal(t, 10, acsvc.Derive(acsvc.Inputs{Buffs: []buffsvc.BuffStatus{off}}).Value, "a buff that does not apply")
}

// ADR 0033 step 10.2: the worn armour's ac powers (its "+N", step 17).
func TestAC_Step10_2_ArmourMagic(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{
		Items:  []itemsvc.Item{armour("chain", 5, itemsvc.StateWorn)},
		Powers: []buffsvc.ItemPower{acPower("chain", -2)},
	})
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermArmourMagic, ItemID: "chain", Value: -2}}, rowsOf(ac, acsvc.TermArmourMagic))
	assert.Equal(t, 3, ac.Value, "chain mail +2")
}

// ADR 0033 step 10.3: minus the worn shield's AC bonus, then its ac powers;
// a body shield gives the AC against missiles too.
func TestAC_Step10_3_Shield(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{
		Items:  []itemsvc.Item{armour("chain", 5, itemsvc.StateWorn), shield("small", 1, 1)},
		Powers: []buffsvc.ItemPower{acPower("small", -1)},
	})
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermShield, ItemID: "small", Value: -1}}, rowsOf(ac, acsvc.TermShield))
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermShieldMagic, ItemID: "small", Value: -1}}, rowsOf(ac, acsvc.TermShieldMagic))
	assert.Equal(t, 3, ac.Value, "chain mail, a small shield +1")
	assert.Nil(t, ac.Missile)

	body := acsvc.Derive(acsvc.Inputs{Items: []itemsvc.Item{armour("chain", 5, itemsvc.StateWorn), shield("body", 1, 2)}})
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermShield, ItemID: "body", Value: -1, MissileValue: ptr(-2)}}, rowsOf(body, acsvc.TermShield))
	assert.Equal(t, 4, body.Value, "against melee")
	assert.Equal(t, ptr(3), body.Missile, "against missiles")

	carried := shield("small", 1, 1)
	carried.State = itemsvc.StateCarried
	assert.Equal(t, 10, acsvc.Derive(acsvc.Inputs{Items: []itemsvc.Item{carried}}).Value, "a carried shield")
}

// ADR 0033 step 10.4: PHB Table 2's defensive adjustment for the sheet's
// Dexterity.
func TestAC_Step10_4_Dexterity(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{Dexterity: 5, DexAdjust: 2})
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermDexterity, Value: 2, Dexterity: 5}}, rowsOf(ac, acsvc.TermDexterity))
	assert.Equal(t, 12, ac.Value, "Dexterity 5 worsens the AC by 2")
}

// ADR 0033 step 10.5: the ac powers of the other equipped pieces, in list
// order; an effect with a weapon-kind condition (the character's own
// attack, ADR 0004 step 18) never moves the AC.
func TestAC_Step10_5_Protection(t *testing.T) {
	cloak := itemsvc.Item{ID: "cloak", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicMisc, State: itemsvc.StateWorn}
	ac := acsvc.Derive(acsvc.Inputs{
		Items: []itemsvc.Item{ring("ring", ""), cloak},
		Powers: []buffsvc.ItemPower{
			acPower("ring", -2),
			acPower("cloak", -1),
			{ItemID: "cloak", PowerID: "melee_only", Effects: buffsvc.BuffEffects{AC: ptr(-4), WeaponKind: buffsvc.WeaponMelee}},
		},
	})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermProtection, ItemID: "ring", Value: -2},
		{Term: acsvc.TermProtection, ItemID: "cloak", Value: -1},
	}, rowsOf(ac, acsvc.TermProtection))
	assert.Equal(t, 7, ac.Value)
}

// ADR 0033 step 10.6: the ac effects of the buffs that apply, in list order;
// one that does not apply (its min_str unmet) is not counted.
func TestAC_Step10_6_Buffs(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{Buffs: []buffsvc.BuffStatus{
		acBuff("Shield of Faith", -2, true),
		acBuff("Gated", -5, false),
		acBuff("Rage", 2, true),
	}})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermBuffs, Buff: "Shield of Faith", Value: -2},
		{Term: acsvc.TermBuffs, Buff: "Rage", Value: 2},
	}, rowsOf(ac, acsvc.TermBuffs))
	assert.Equal(t, 10, ac.Value)
}

// ADR 0033 step 10.7: never better than -10 (DMG p.181), against missiles
// too.
func TestAC_Step10_7_Floor(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{
		Items:     []itemsvc.Item{armour("plate", 1, itemsvc.StateWorn), shield("body", 1, 2)},
		Powers:    []buffsvc.ItemPower{acPower("plate", -5)},
		Buffs:     []buffsvc.BuffStatus{acBuff("Many wards", -4, true)},
		DexAdjust: -4,
	})
	assert.Equal(t, -10, ac.Value, "1 - 5 - 1 - 4 - 4 = -13")
	assert.True(t, ac.Floored)
	assert.Nil(t, ac.Missile, "-14 floored is -10 too: one AC")

	at := acsvc.Derive(acsvc.Inputs{Items: []itemsvc.Item{armour("plate", -10, itemsvc.StateWorn)}})
	assert.Equal(t, -10, at.Value)
	assert.False(t, at.Floored, "-10 itself is not floored")
}

// ADR 0033 step 12: a without_magic_armour effect counts only while the
// worn body armour has no ac power (the ring of protection with magic
// armour); its other effects are not conditioned. A buff's is conditioned
// the same way.
func TestAC_Step12_WithoutMagicArmour(t *testing.T) {
	ringPower := buffsvc.ItemPower{ItemID: "ring", PowerID: "magic", Effects: buffsvc.BuffEffects{
		AC: ptr(-2), SaveBonus: ptr(2), WithoutMagicArmour: true,
	}}
	plain := acsvc.Derive(acsvc.Inputs{
		Items:  []itemsvc.Item{armour("chain", 5, itemsvc.StateWorn), ring("ring", "")},
		Powers: []buffsvc.ItemPower{ringPower},
	})
	assert.Equal(t, 3, plain.Value, "plain chain mail: the ring counts")

	magic := acsvc.Derive(acsvc.Inputs{
		Items:  []itemsvc.Item{armour("chain", 5, itemsvc.StateWorn), ring("ring", "")},
		Powers: []buffsvc.ItemPower{acPower("chain", -1), ringPower},
		Buffs: []buffsvc.BuffStatus{{BuffEffect: buffsvc.BuffEffect{Name: "Ward", Effects: buffsvc.BuffEffects{
			AC: ptr(-1), WithoutMagicArmour: true,
		}}, Applies: true}},
	})
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermProtection, ItemID: "ring", Value: -2, Off: acsvc.OffMagicArmour}}, rowsOf(magic, acsvc.TermProtection))
	assert.Equal(t, []acsvc.Row{{Term: acsvc.TermBuffs, Buff: "Ward", Value: -1, Off: acsvc.OffMagicArmour}}, rowsOf(magic, acsvc.TermBuffs))
	assert.Equal(t, 4, magic.Value, "chain mail +1: neither the ring nor the ward adds")
}

// ADR 0033 step 12: two worn pieces of one catalogue code with an ac
// effect, only the best counts (DMG p.149's two rings), the other is shown
// as not adding; pieces of no code or of other codes all count.
func TestAC_Step12_OneOfTwoPiecesOfACodeCounts(t *testing.T) {
	ac := acsvc.Derive(acsvc.Inputs{
		Items: []itemsvc.Item{
			ring("ring1", "ring_of_protection"),
			ring("ring3", "ring_of_protection"),
			ring("homebrew", ""),
			ring("other", "cloak_of_protection"),
		},
		Powers: []buffsvc.ItemPower{
			acPower("ring1", -1), acPower("ring3", -3), acPower("homebrew", -1), acPower("other", -1),
		},
	})
	assert.Equal(t, []acsvc.Row{
		{Term: acsvc.TermProtection, ItemID: "ring1", Value: -1, Off: acsvc.OffDuplicate},
		{Term: acsvc.TermProtection, ItemID: "ring3", Value: -3},
		{Term: acsvc.TermProtection, ItemID: "homebrew", Value: -1},
		{Term: acsvc.TermProtection, ItemID: "other", Value: -1},
	}, rowsOf(ac, acsvc.TermProtection))
	assert.Equal(t, 5, ac.Value, "10 - 3 - 1 - 1")
}

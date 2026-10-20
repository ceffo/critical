package ruletablesadpt_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// equipmentOf parses the fixture's rule tables and equipment with mutate
// applied to the decoded rule tables and equipment objects first.
func equipmentOf(t *testing.T, mutate func(rules, equipment map[string]any)) (*ruletablesvc.Tables, error) {
	t.Helper()
	var rules, equipment map[string]any
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &rules))
	require.NoError(t, json.Unmarshal(fixtureEquipment(t), &equipment))
	mutate(rules, equipment)
	rawRules, err := json.Marshal(rules)
	require.NoError(t, err)
	var rawEquipment []byte
	if equipment != nil {
		rawEquipment, err = json.Marshal(equipment)
		require.NoError(t, err)
	}
	return ruletablesadpt.Parse(rawRules, rawEquipment, fixtureLanguages(t))
}

func list(d map[string]any, key string) []any { return d[key].([]any) }

func rowOf(d map[string]any, key string, i int) map[string]any {
	return list(d, key)[i].(map[string]any)
}

func weapon(t *testing.T, e *ruletablesvc.Equipment, code string) ruletablesvc.Weapon {
	t.Helper()
	for _, w := range e.Weapons {
		if w.Code == code {
			return w
		}
	}
	require.Failf(t, "no such weapon", "%s", code)
	return ruletablesvc.Weapon{}
}

// ADR 0021 step 13.1 (ADR 0033 steps 21 and 23): the artifact hands on the
// equipment rows, copied cell for cell from PHB Table 44, and Tables 52 and 56.
// Expected values are read from the PHB pages, not derived from the artifact.
func TestParse_Step13_1_ReadsFormat7Equipment(t *testing.T) {
	tables, err := equipmentOf(t, func(_, _ map[string]any) {})
	require.NoError(t, err)
	e, ok := tables.Equipment()
	require.True(t, ok)
	assert.Len(t, e.Weapons, 63)
	assert.Len(t, e.Ammunition, 9)
	assert.Len(t, e.Armour, 15)
	assert.Len(t, e.Shields, 4)

	ls := weapon(t, e, "long_sword") // PHB Table 44: M, S, speed 5, 1d8 / 1d12
	assert.Equal(t, ruletablesvc.Weapon{
		Code: "long_sword", Name: ls.Name, WeaponType: "long_sword", Kind: "melee", Size: "m", Types: []string{"s"},
		Speed: 5, DamageSM: "1d8", DamageL: "1d12",
	}, ls)
	assert.Equal(t, "Fixture long sword", ls.Name["en"])
	bow := weapon(t, e, "long_bow") // a launcher: a speed, no damage, fires arrows
	assert.Equal(t, []string{"arrow"}, bow.Fires)
	assert.Empty(t, bow.DamageSM)
	assert.True(t, bow.TwoHanded)
	assert.Equal(t, []string{"blowgun_dart", "needle"}, weapon(t, e, "blowgun").Fires)
	bec := weapon(t, e, "bec_de_corbin")
	assert.Equal(t, []string{"p", "b"}, bec.Types, "a two-type weapon keeps both types")
	assert.Equal(t, 13, weapon(t, e, "awl_pike").Speed)
	assert.Equal(t, "1d3-1", weapon(t, e, "jousting_lance").DamageSM)

	byCode := map[string]ruletablesvc.Armour{}
	for _, a := range e.Armour {
		byCode[a.Code] = a
	}
	// Table 46 without a shield, Table 29's columns, Table 52's rows.
	assert.Equal(t, 8, byCode["leather"].BaseAC)
	assert.Equal(t, 5, byCode["chain_mail"].BaseAC)
	assert.Equal(t, "metal", byCode["chain_mail"].SkillColumn)
	assert.Equal(t, "elven_chain", byCode["elven_chain_mail"].SkillColumn)
	assert.Equal(t, "chain", byCode["elven_chain_mail"].ArmorRow)
	assert.Equal(t, 1, byCode["full_plate"].BaseAC)
	assert.Equal(t, "none_allowed", byCode["full_plate"].SkillColumn)
	body := e.Shields[3]
	assert.Equal(t, "body", body.Kind)
	assert.Equal(t, 1, body.ACMelee)
	assert.Equal(t, 2, body.ACMissile, "the body shield betters AC by 2 against missiles")

	vs, ok := tables.WeaponVsArmor()
	require.True(t, ok)
	plate, _ := vs.Row("plate")
	assert.Equal(t, ruletablesvc.WeaponVsArmorRow{ArmorRow: "plate", Slash: 3}, plate, "Table 52: plate +3 against slashing")
	leather, _ := vs.Row("leather")
	assert.Equal(t, -2, leather.Pierce, "Table 52: leather -2 against piercing")
	init, ok := tables.InitiativeItems()
	require.True(t, ok)
	potion, _ := init.Row("potion")
	assert.Equal(t, ruletablesvc.InitiativeItemRow{Kind: "potion", Modifier: 4, HasModifier: true}, potion, "Table 56: potion +4")
	wand, _ := init.Row("wand")
	assert.Equal(t, 3, wand.Modifier)
	scroll, _ := init.Row("scroll")
	assert.False(t, scroll.HasModifier, "a scroll takes the spell's casting time, no modifier")
}

// ADR 0021 step 13.2: the equipment and its two tables are all required.
func TestParse_Step13_2_EquipmentIsRequired(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(rules, equipment map[string]any)
		want   string
	}{
		{"no weapon_vs_armor", func(r, _ map[string]any) { delete(r, "weapon_vs_armor") }, "the artifact needs the weapon_vs_armor table"},
		{"no initiative_item", func(r, _ map[string]any) { delete(r, "initiative_item") }, "the artifact needs the initiative_item table"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := equipmentOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
	_, err := ruletablesadpt.Parse(fixtureRuleTables(t), nil, fixtureLanguages(t))
	require.ErrorContains(t, err, "the artifact needs the equipment object")
}

// ADR 0021 step 13.3: the rules the schema cannot express are the
// importer's, each refusal naming the table or row.
func TestParse_Step13_3_RefusesBrokenEquipment(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(rules, equipment map[string]any)
		want   string
	}{
		{"a code twice", func(_, e map[string]any) {
			rowOf(e, "weapons", 1)["code"] = rowOf(e, "weapons", 0)["code"]
		}, `equipment: weapons: code "battle_axe" twice`},
		{"a code shared by a weapon and a shield", func(_, e map[string]any) {
			rowOf(e, "shields", 0)["code"] = rowOf(e, "weapons", 0)["code"]
		}, `shields: code "battle_axe" twice`},
		{"a launcher firing an ammunition kind no row carries", func(_, e map[string]any) {
			e["ammunition"] = list(e, "ammunition")[:1] // only the blowgun's dart is left
		}, `fires "needle", which no ammunition row carries`},
		{"a launcher with damage of its own", func(_, e map[string]any) {
			w := rowOf(e, "weapons", 2) // composite long bow
			w["damage_sm"], w["damage_l"] = "1d6", "1d6"
		}, "a launcher is a missile weapon with no damage of its own"},
		{"a melee weapon with a damage type and no damage", func(_, e map[string]any) {
			w := rowOf(e, "weapons", 0)
			delete(w, "damage_sm")
			delete(w, "damage_l")
		}, `weapon "battle_axe": no damage and no ammunition kind`},
		{"a Table 52 row twice", func(r, _ map[string]any) {
			rowOf(r, "weapon_vs_armor", 1)["armor_row"] = "banded"
		}, `weapon_vs_armor table: "banded" twice`},
		{"a Table 52 row missing", func(r, _ map[string]any) {
			r["weapon_vs_armor"] = list(r, "weapon_vs_armor")[1:]
		}, `weapon_vs_armor table: missing "banded"`},
		{"a scroll with a modifier", func(r, _ map[string]any) {
			rowOf(r, "initiative_item", 1)["modifier"] = 3
		}, "a scroll has no modifier"},
		{"a wand without a modifier", func(r, _ map[string]any) {
			delete(rowOf(r, "initiative_item", 5), "modifier")
		}, `initiative_item table: "wand": no modifier`},
		{"an unknown equipment field", func(_, e map[string]any) {
			rowOf(e, "armour", 0)["weight"] = 35
		}, "weight"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := equipmentOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The audit of ADR 0030 steps 17 and 18: every term of every thief and bard
// skill, for every race, every Dexterity from 9 to 25 and every body armour
// column, against the book. The expected values are the book's tables written
// out below by hand (PHB p.39 Tables 26 to 29, p.42 Table 33), never read from
// the catalogue or computed by the code under test. The armour readings (which
// Table 29 column each body armour reads) are the app's decided ones (ADR 0033
// step 14 on ADR 0030 steps 17 and 18), written out per class.

// thiefBase is PHB Table 26, in its order.
var thiefBase = []struct {
	skill string
	base  int
}{
	{"pick_pockets", 15},
	{"open_locks", 10},
	{"find_remove_traps", 5},
	{"move_silently", 10},
	{"hide_in_shadows", 5},
	{"detect_noise", 15},
	{"climb_walls", 60},
	{"read_languages", 0},
}

// bardBase is PHB Table 33, in its order.
var bardBase = []struct {
	skill string
	base  int
}{
	{"climb_walls", 50}, {"detect_noise", 20}, {"pick_pockets", 10}, {"read_languages", 5},
}

// bookRace is PHB Table 27 (a dash is 0); the human has no column: 0 on
// every skill.
var bookRace = map[string]map[string]int{
	"human": {},
	"dwarf": {"open_locks": 10, "find_remove_traps": 15, "climb_walls": -10, "read_languages": -5},
	"elf": {
		"pick_pockets": 5, "open_locks": -5, "move_silently": 5, "hide_in_shadows": 10,
		"detect_noise": 5,
	},
	"gnome": {
		"open_locks": 5, "find_remove_traps": 10, "move_silently": 5, "hide_in_shadows": 5,
		"detect_noise": 10, "climb_walls": -15,
	},
	"half_elf": {"pick_pockets": 10, "hide_in_shadows": 5},
	"halfling": {
		"pick_pockets": 5, "open_locks": 5, "find_remove_traps": 5, "move_silently": 10,
		"hide_in_shadows": 15, "detect_noise": 5, "climb_walls": -15, "read_languages": -5,
	},
}

// bookDex is PHB Table 28 for every Dexterity from 9 to 25, a row per score
// (13 to 15 one band in the book; a dash is 0). Its columns are Pick
// Pockets, Open Locks, Find/Remove Traps, Move Silently and Hide in Shadows;
// any other skill adjusts by 0. The book stops at 19: 20 to 25 continue the
// row of 19 (ADR 0030 step 17, the user's ruling of 2026-11-15).
var bookDex = map[int][5]int{
	9:  {-15, -10, -10, -20, -10},
	10: {-10, -5, -10, -15, -5},
	11: {-5, 0, -5, -10, 0},
	12: {0, 0, 0, -5, 0},
	13: {0, 0, 0, 0, 0},
	14: {0, 0, 0, 0, 0},
	15: {0, 0, 0, 0, 0},
	16: {0, 5, 0, 0, 0},
	17: {5, 10, 0, 5, 5},
	18: {10, 15, 5, 10, 10},
	19: {15, 20, 10, 15, 15},
	20: {15, 20, 10, 15, 15},
	21: {15, 20, 10, 15, 15},
	22: {15, 20, 10, 15, 15},
	23: {15, 20, 10, 15, 15},
	24: {15, 20, 10, 15, 15},
	25: {15, 20, 10, 15, 15},
}

// dexColumns are Table 28's columns, in bookDex's order.
var dexColumns = []string{"pick_pockets", "open_locks", "find_remove_traps", "move_silently", "hide_in_shadows"}

// PHB Table 29's columns (a dash is 0); leather is the baseline, 0 on every
// skill.
var (
	table29None  = map[string]int{"pick_pockets": 5, "move_silently": 10, "hide_in_shadows": 5, "climb_walls": 10}
	table29Elven = map[string]int{
		"pick_pockets": -20, "open_locks": -5, "find_remove_traps": -5, "move_silently": -10,
		"hide_in_shadows": -10, "detect_noise": -5, "climb_walls": -20,
	}
	table29Padded = map[string]int{
		"pick_pockets": -30, "open_locks": -10, "find_remove_traps": -10, "move_silently": -20,
		"hide_in_shadows": -20, "detect_noise": -10, "climb_walls": -30,
	}
	table29Leather = map[string]int{}
)

// bardMetal is Table 29's elven chain column plus its note's -5 for a bard
// in non-elven metal armour up to chain mail, on every bard skill.
var bardMetal = map[string]int{"climb_walls": -25, "detect_noise": -10, "pick_pockets": -25, "read_languages": -5}

// The Table 29 column each body armour column reads (ADR 0033 step 14):
// none worn reads No Armor; metal (ring, scale, chain, hide, brigandine) and
// none_allowed (banded, splint, plate) read padded or studded leather for a
// thief, the armour it may not wear (PHB p.39 lists none); a bard in metal
// reads elven chain then -5 (Table 29's note), in none_allowed the padded or
// studded column.
var (
	thiefArmour = map[string]map[string]int{
		"": table29None, ruletablesvc.ColumnNone: table29None, ruletablesvc.ColumnLeather: table29Leather,
		ruletablesvc.ColumnPaddedStudded: table29Padded, ruletablesvc.ColumnElvenChain: table29Elven,
		ruletablesvc.ColumnMetal: table29Padded, ruletablesvc.ColumnNoneAllowed: table29Padded,
	}
	bardArmour = map[string]map[string]int{
		"": table29None, ruletablesvc.ColumnNone: table29None, ruletablesvc.ColumnLeather: table29Leather,
		ruletablesvc.ColumnPaddedStudded: table29Padded, ruletablesvc.ColumnElvenChain: table29Elven,
		ruletablesvc.ColumnMetal: bardMetal, ruletablesvc.ColumnNoneAllowed: table29Padded,
	}
)

// bookDexOf is skill's Table 28 cell at dex (0 for a skill without a column).
func bookDexOf(dex int, skill string) int {
	for i, c := range dexColumns {
		if c == skill {
			return bookDex[dex][i]
		}
	}
	return 0
}

// auditSkills checks every term of class's skills against base, the book's
// tables and armour, for every race, Dexterity 9 to 25 and body armour
// column, with points (one value on every skill): each term is its table's
// cell, Raw their sum, Percent Raw capped at 95 and floored at 0.
func auditSkills(t *testing.T, class string, base []struct {
	skill string
	base  int
}, armour map[string]map[string]int, points int,
) {
	t.Helper()
	tbl := phb(t)
	rs := rogueOf(t, class)
	require.NotNil(t, rs)
	pts := map[string]int{}
	for _, b := range base {
		pts[b.skill] = points
	}
	for race, raceAdj := range bookRace {
		for dex := 9; dex <= 25; dex++ {
			for column, armourAdj := range armour {
				got, err := tbl.RogueSkills(*rs, race, dex, ruletablesvc.SkillArmorFor(rs, column), pts)
				require.NoError(t, err, "%s %s dex %d armour %q", class, race, dex, column)
				require.Len(t, got, len(base))
				for i, b := range base {
					where := []any{"%s %s, %s, Dexterity %d, armour %q, %d points", class, b.skill, race, dex, column, points}
					s := got[i]
					want := ruletablesvc.SkillPercent{
						Skill: b.skill, Base: b.base, Points: points, Race: raceAdj[b.skill],
						Dexterity: bookDexOf(dex, b.skill), Armor: armourAdj[b.skill],
					}
					want.Raw = want.Base + want.Points + want.Race + want.Dexterity + want.Armor
					want.Percent = min(max(want.Raw, 0), 95)
					assert.Equal(t, want, s, where...)
				}
			}
		}
	}
}

// ADR 0030 step 17: each of the eight thief skills is Table 26's base + the
// points + Table 27 + Table 28 (Dexterity 20 and more: the row of 19) + Table
// 29's column the worn armour reads, capped at 95, floored at 0; with no
// points (the floor reached, e.g. Move Silently at Dexterity 9 in padded
// leather) and 80 on every skill (the cap reached).
func TestThiefSkills_Step17_EveryTermAgainstTheBook(t *testing.T) {
	for _, points := range []int{0, 80} {
		auditSkills(t, "thief", thiefBase, thiefArmour, points)
	}
}

// ADR 0030 step 18: the bard's four skills are Table 33's base + the points +
// Table 27 + Table 28 (Pick Pockets only; Dexterity 20 and more: the row of
// 19) + Table 29's column the worn armour reads (metal: elven chain and -5),
// capped at 95, floored at 0.
func TestBardSkills_Step18_EveryTermAgainstTheBook(t *testing.T) {
	for _, points := range []int{0, 80} {
		auditSkills(t, "bard", bardBase, bardArmour, points)
	}
}

// ADR 0030 step 17: Dexterity 20 to 25 continue Table 28's row of 19 (the
// user's ruling; the book stops at 19), spelled out for the five skills.
func TestThiefSkills_Step17_Dexterity20AndMoreReadsTheRowOf19(t *testing.T) {
	tbl := phb(t)
	for dex := 19; dex <= 25; dex++ {
		got, err := rogueSkills(t, tbl, "thief", "human", dex, ruletablesvc.ColumnLeather, nil)
		require.NoError(t, err)
		assert.Equal(t, []int{15, 20, 10, 15, 15, 0, 0, 0}, dexterities(got), "Dexterity %d", dex)
	}
}

// ADR 0030 step 17: under 9 (below the thief's and the bard's minimums, PHB
// p.39 and p.42; reachable on a stored sheet) Table 28 has no row: the
// Dexterity term is 0, so Dexterity 8 reads better than 9. Kept as it is
// until the user rules on it.
func TestRogueSkills_Step17_DexterityUnder9AdjustsByNothing(t *testing.T) {
	tbl := phb(t)
	for dex := 1; dex <= 8; dex++ {
		thief, err := rogueSkills(t, tbl, "thief", "human", dex, ruletablesvc.ColumnLeather, nil)
		require.NoError(t, err)
		assert.Equal(t, []int{0, 0, 0, 0, 0, 0, 0, 0}, dexterities(thief), "thief, Dexterity %d", dex)
		assert.Equal(t, []int{15, 10, 5, 10, 5, 15, 60, 0}, percents(thief), "thief, Dexterity %d", dex)
		bard, err := rogueSkills(t, tbl, "bard", "human", dex, ruletablesvc.ColumnLeather, nil)
		require.NoError(t, err)
		assert.Equal(t, []int{50, 20, 10, 5}, percents(bard), "bard, Dexterity %d", dex)
	}
}

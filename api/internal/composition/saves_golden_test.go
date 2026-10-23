package composition_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// savesRow is one character's save figure as the view serves it: the five
// targets, the race's bonus per category, Wisdom's magical defence and the
// class save_bonus grants.
type savesRow struct {
	Name         string                        `json:"name"`
	Saves        charactersvc.Saves            `json:"saves"`
	RaceBonus    charactersvc.Saves            `json:"raceBonus"`
	MagicDefence int                           `json:"magicDefence"`
	ClassBonuses []charactersvc.ClassSaveBonus `json:"classBonuses"`
}

// savesRows derives every character through the view of a fresh table, in
// order.
func savesRows(t *testing.T, chars []charactersvc.Character) []byte {
	t.Helper()
	ctx := context.Background()
	bs, m := dexTable(t)
	rows := make([]savesRow, 0, len(chars))
	for _, c := range chars {
		id, err := m.AddCharacter(ctx, c)
		require.NoError(t, err)
		v, err := bs.Character.View(ctx, id)
		require.NoError(t, err, c.Name)
		rows = append(rows, savesRow{Name: c.Name, Saves: v.Saves, RaceBonus: v.RaceSaveBonus, MagicDefence: v.MagicDefence, ClassBonuses: v.ClassSaveBonuses})
	}
	out, err := json.Marshal(rows)
	require.NoError(t, err)
	return out
}

func savesCharacter(name, race string, wis, con int, es ...classes.Entry) charactersvc.Character {
	c := charactersvc.Character{
		Name: name, Race: race, Str: 12, Dex: 12, Con: con, Int: 12, Wis: wis, Cha: 12, HPMax: 30,
		Class: es[0].Class, RealizedLevel: es[0].RealizedLevel, XPTotal: es[0].StartingXP,
	}
	if len(es) > 1 {
		c.Classes = es
	}
	return c
}

// ADR 0046 step 3.5 (step 7: the values are the old code's, committed in
// testdata/golden/saves_rows.json): every class of the fixture catalogue at
// every level from 1 to 20 (so every group's level bands of Table 60), the
// best-of of multi-class characters and of dual-class ones before and past
// the crossover (ADR 0042 steps 9 and 13), with the class save_bonus grants
// and Wisdom's magical defence, give the same save figure wherever the rule
// that builds it lives.
func TestSaves_Step3_GoldenRows(t *testing.T) {
	var chars []charactersvc.Character
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard", "psionicist"} {
		for level := 1; level <= 20; level++ {
			chars = append(chars, savesCharacter(fmt.Sprintf("%s %d", class, level), "human", 10, 12, classes.SingleEntry(class, level, 0)))
		}
	}
	frozen := func(class string, level int) classes.Entry {
		return classes.Entry{Class: class, RealizedLevel: level, StartingXP: 250_000, State: classes.EntryFrozen, LeftAtLevel: level}
	}
	chars = append(chars,
		savesCharacter("elf fighter 5 / mage 4", "elf", 10, 12, classes.SingleEntry("fighter", 5, 0), classes.SingleEntry("mage", 4, 0)),
		savesCharacter("half-elf fighter 7 / cleric 6 / mage 5", "half_elf", 15, 12,
			classes.SingleEntry("fighter", 7, 0), classes.SingleEntry("cleric", 6, 0), classes.SingleEntry("mage", 5, 0)),
		savesCharacter("dwarf fighter 4 / thief 5", "dwarf", 10, 15, classes.SingleEntry("fighter", 4, 0), classes.SingleEntry("thief", 5, 0)),
		savesCharacter("human thief 15 / psionicist 14", "human", 17, 12, classes.SingleEntry("thief", 15, 0), classes.SingleEntry("psionicist", 14, 0)),
		savesCharacter("elf fighter 9 dual mage 7 (before the crossover)", "elf", 10, 12, frozen("fighter", 9), classes.SingleEntry("mage", 7, 0)),
		savesCharacter("elf fighter 9 dual mage 10 (past the crossover)", "elf", 10, 12, frozen("fighter", 9), classes.SingleEntry("mage", 10, 0)),
		savesCharacter("human mage 9 dual cleric 5 (before the crossover)", "human", 3, 12, frozen("mage", 9), classes.SingleEntry("cleric", 5, 0)),
		savesCharacter("human mage 9 dual cleric 10 (past the crossover)", "human", 18, 12, frozen("mage", 9), classes.SingleEntry("cleric", 10, 0)),
		savesCharacter("human psionicist 9 dual fighter 5 (before the crossover)", "human", 12, 12, frozen("psionicist", 9), classes.SingleEntry("fighter", 5, 0)),
		savesCharacter("human psionicist 9 dual fighter 10 (past the crossover)", "human", 12, 12, frozen("psionicist", 9), classes.SingleEntry("fighter", 10, 0)),
	)
	golden(t, "saves_rows.json", savesRows(t, chars))
}

// ADR 0046 step 3.5: the race's Constitution bonus (ADR 0030 step 22.1) per
// category, for every race of the fixture catalogue across PHB Table 9's
// rows and either side of them, is the old code's (committed in
// testdata/golden/saves_race_bonus.json).
func TestSaves_Step3_RaceBonusPerCategory(t *testing.T) {
	var chars []charactersvc.Character
	for _, race := range []string{"dwarf", "elf", "gnome", "half_elf", "halfling", "human"} {
		for _, con := range []int{3, 4, 6, 7, 10, 11, 13, 14, 17, 18, 19, 20} {
			chars = append(chars, savesCharacter(fmt.Sprintf("%s con %d", race, con), race, 10, con, classes.SingleEntry("fighter", 1, 0)))
		}
	}
	golden(t, "saves_race_bonus.json", savesRows(t, chars))
}

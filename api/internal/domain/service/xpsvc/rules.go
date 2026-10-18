package xpsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// BonusPercent is the prime-requisite experience bonus (ADR 0015 step 1).
const BonusPercent = 10

// BonusMinScore is the score both prime requisites need for the bonus (ADR
// 0015 step 2).
const BonusMinScore = 16

// LevelCells is how many levels of Table 25 the ledger lists around the
// sheet's level (ADR 0015 step 3).
const LevelCells = 8

// EntryAmount is ADR 0015 step 1 for one entry: the bonus is BonusPercent of
// the base rounded to the nearest point, halves up, and the entry adds base
// plus bonus. Without the bonus the entry adds its base.
func EntryAmount(base int, bonus bool) Amount {
	a := Amount{Base: base, Total: base}
	if bonus {
		a.BonusAmount = (base*BonusPercent + 50) / 100
		a.Total += a.BonusAmount
	}
	return a
}

// LedgerTotal is ADR 0015 step 1 for the ledger: the sheet's starting XP
// plus every entry's amount.
func LedgerTotal(opening int, entries []Entry) int {
	total := opening
	for _, e := range entries {
		total += EntryAmount(e.Base, e.Bonus).Total
	}
	return total
}

// BonusEligible is ADR 0015 step 2: the bard's prime requisites, Dexterity
// and Charisma, are both at least BonusMinScore on the sheet.
func BonusEligible(dex, cha int) bool {
	return dex >= BonusMinScore && cha >= BonusMinScore
}

// ComputeProgress is ADR 0015 step 3, read in class's experience table
// (ADR 0030 step 3: PHB Tables 14, 20, 23 and 25): the floor is the
// threshold of the level, the ceiling the next level's. ToNext is what is
// left to the ceiling (0 once reached), Progress the share of the way from
// floor to ceiling clamped to 0..1. At the last level there is no ceiling
// and the progress is full; a level above the table is treated as the last
// level, keeping its own Level. The cells are LevelCells consecutive levels
// starting two below the level, kept inside the table.
func ComputeProgress(class string, level, total int, table Thresholds) (Progress, error) {
	threshold := func(l int) (int, error) {
		th, err := table.XPThreshold(class, l)
		if err != nil {
			return 0, fmt.Errorf("xp progress: %w", err)
		}
		return th, nil
	}
	floor, err := threshold(min(level, ruletablesvc.MaxLevel))
	if err != nil {
		return Progress{}, err
	}
	p := Progress{Level: level, CurrentFloor: floor, Progress: 1, ProgressPercent: 100}
	if level < ruletablesvc.MaxLevel {
		ceiling, err := threshold(level + 1)
		if err != nil {
			return Progress{}, err
		}
		p.NextLevel = level + 1
		p.NextThreshold = &ceiling
		p.ToNext = max(0, ceiling-total)
		p.Progress = min(1, max(0, float64(total-floor)/float64(ceiling-floor)))
		p.ProgressPercent = min(100, max(0, (total-floor)*100/(ceiling-floor)))
	}
	start := min(max(1, level-2), ruletablesvc.MaxLevel-LevelCells+1)
	for l := start; l < start+LevelCells; l++ {
		th, err := threshold(l)
		if err != nil {
			return Progress{}, err
		}
		p.Levels = append(p.Levels, LevelCell{Level: l, Threshold: th, Status: cellStatus(l, level)})
	}
	return p, nil
}

func cellStatus(l, level int) CellStatus {
	switch {
	case l < level:
		return CellPast
	case l == level:
		return CellCurrent
	case l == level+1:
		return CellNext
	default:
		return CellAhead
	}
}

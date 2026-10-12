// Package ruletablesvc holds the AD&D 2e Player's Handbook rule tables the
// engine looks values up in: Table 1 (Strength), Table 2 (Dexterity) and
// Table 32 (Bard spell progression), plus the validation that makes a table
// usable (contiguous, no overlap). The seed adapter builds a Tables from
// seeds/rule_tables.json; consumers (attack resolution, spell preparation)
// depend on Lookup only.
package ruletablesvc

import (
	"errors"
	"fmt"
)

// MaxSpellLevel is the highest bard spell level in Table 32.
const MaxSpellLevel = 6

// MaxBardLevel is the last level of Table 32.
const MaxBardLevel = 20

// ExceptionalMax is the exceptional percentile of 18/00. Exceptional strength
// is the percentile of "18/xx": 0 for a plain 18, 1-99
// for 18/01-18/99, ExceptionalMax for 18/00.
const ExceptionalMax = 100

// Lookup is the read port over the PHB rule tables.
type Lookup interface {
	// StrengthMods returns the PHB Table 1 hit-probability and damage
	// adjustments. exceptional only matters when str is 18.
	StrengthMods(str, exceptional int) (hit, dmg int, err error)
	// DexMods returns the PHB Table 2 missile attack adjustment and the
	// defensive adjustment to AC (negative = armour class improves).
	DexMods(dex int) (missile, ac int, err error)
	// BardSpellsPerDay returns the spells a bard of that level may cast per
	// day, keyed by spell level. Spell levels with no slot are absent.
	BardSpellsPerDay(level int) (map[int]int, error)
}

// StrengthRow is one row of PHB Table 1. ExcMin/ExcMax are the 18/xx range
// (0/0 for rows with no exceptional part).
type StrengthRow struct {
	StrMin, StrMax int
	ExcMin, ExcMax int
	Hit, Dmg       int
}

// DexRow is one row of PHB Table 2.
type DexRow struct {
	DexMin, DexMax int
	Reaction       int
	Missile        int
	Defensive      int
}

// BardRow is one row of PHB Table 32: PerSpellLevel[i] is the number of
// spells of spell level i+1.
type BardRow struct {
	Level         int
	PerSpellLevel [MaxSpellLevel]int
}

// Tables implements Lookup.
type Tables struct {
	str  []StrengthRow
	dex  []DexRow
	bard map[int]BardRow
}

var _ Lookup = (*Tables)(nil)

// New validates the rows and builds the lookup. Strength rows must cover
// scores 1..25 with 18 split into a contiguous 0..100 exceptional range;
// dexterity rows 1..25 contiguously; bard rows levels 1..20 exactly once.
func New(str []StrengthRow, dex []DexRow, bard []BardRow) (*Tables, error) {
	if err := validateStrength(str); err != nil {
		return nil, fmt.Errorf("strength table: %w", err)
	}
	if err := validateDex(dex); err != nil {
		return nil, fmt.Errorf("dexterity table: %w", err)
	}
	byLevel := make(map[int]BardRow, len(bard))
	for _, r := range bard {
		if r.Level < 1 || r.Level > MaxBardLevel {
			return nil, fmt.Errorf("bard table: level %d out of range 1..%d", r.Level, MaxBardLevel)
		}
		if _, dup := byLevel[r.Level]; dup {
			return nil, fmt.Errorf("bard table: duplicate level %d", r.Level)
		}
		for i, n := range r.PerSpellLevel {
			if n < 0 {
				return nil, fmt.Errorf("bard table: level %d spell level %d: negative count", r.Level, i+1)
			}
		}
		byLevel[r.Level] = r
	}
	for l := 1; l <= MaxBardLevel; l++ {
		if _, ok := byLevel[l]; !ok {
			return nil, fmt.Errorf("bard table: missing level %d", l)
		}
	}
	return &Tables{
		str:  append([]StrengthRow(nil), str...),
		dex:  append([]DexRow(nil), dex...),
		bard: byLevel,
	}, nil
}

func validateStrength(rows []StrengthRow) error {
	next := 1 // next expected str score
	exc := 0  // next expected exceptional percentile within score 18
	for i, r := range rows {
		if r.StrMin > r.StrMax || r.ExcMin > r.ExcMax {
			return fmt.Errorf("row %d: min above max", i)
		}
		if r.StrMin != r.StrMax && (r.ExcMin != 0 || r.ExcMax != 0) {
			return fmt.Errorf("row %d: exceptional range on a multi-score row", i)
		}
		if r.StrMin != 18 && (r.ExcMin != 0 || r.ExcMax != 0) {
			return fmt.Errorf("row %d: exceptional range outside STR 18", i)
		}
		if r.StrMin == 18 {
			if next != 18 || r.StrMax != 18 || r.ExcMin != exc || r.ExcMax > ExceptionalMax {
				return fmt.Errorf("row %d: STR 18 exceptional ranges must be contiguous from 0 to %d", i, ExceptionalMax)
			}
			exc = r.ExcMax + 1
			if r.ExcMax == ExceptionalMax {
				next = 19
			}
			continue
		}
		if r.StrMin != next {
			return fmt.Errorf("row %d: score %d expected, got %d", i, next, r.StrMin)
		}
		next = r.StrMax + 1
	}
	if next != 26 {
		return errors.New("must cover scores 1..25 (with 18/00 as the last 18 row)")
	}
	return nil
}

func validateDex(rows []DexRow) error {
	next := 1
	for i, r := range rows {
		if r.DexMin > r.DexMax {
			return fmt.Errorf("row %d: min above max", i)
		}
		if r.DexMin != next {
			return fmt.Errorf("row %d: score %d expected, got %d", i, next, r.DexMin)
		}
		next = r.DexMax + 1
	}
	if next != 26 {
		return errors.New("must cover scores 1..25")
	}
	return nil
}

// StrengthMods implements Lookup.
func (t *Tables) StrengthMods(str, exceptional int) (hit, dmg int, err error) {
	if exceptional < 0 || exceptional > ExceptionalMax {
		return 0, 0, fmt.Errorf("exceptional strength %d out of range 0..%d", exceptional, ExceptionalMax)
	}
	if str != 18 {
		exceptional = 0
	}
	for _, r := range t.str {
		if str >= r.StrMin && str <= r.StrMax && exceptional >= r.ExcMin && exceptional <= r.ExcMax {
			return r.Hit, r.Dmg, nil
		}
	}
	return 0, 0, fmt.Errorf("strength %d out of range 1..25", str)
}

// DexMods implements Lookup.
func (t *Tables) DexMods(dex int) (missile, ac int, err error) {
	for _, r := range t.dex {
		if dex >= r.DexMin && dex <= r.DexMax {
			return r.Missile, r.Defensive, nil
		}
	}
	return 0, 0, fmt.Errorf("dexterity %d out of range 1..25", dex)
}

// BardSpellsPerDay implements Lookup.
func (t *Tables) BardSpellsPerDay(level int) (map[int]int, error) {
	r, ok := t.bard[level]
	if !ok {
		return nil, fmt.Errorf("bard level %d out of range 1..%d", level, MaxBardLevel)
	}
	out := make(map[int]int, MaxSpellLevel)
	for i, n := range r.PerSpellLevel {
		if n > 0 {
			out[i+1] = n
		}
	}
	return out, nil
}

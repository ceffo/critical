// Package ruletablesvc holds the AD&D 2e Player's Handbook rule tables the
// engine looks values up in: Table 1 (Strength), Table 2 (Dexterity) and
// Table 32 (Bard spell progression) and Table 25 (the Thief/Bard experience
// levels), plus the validation that makes a table
// usable (contiguous, no overlap). The ruletablesadpt adapter builds a Tables
// from the catalogue artifact's rule_tables (ADR 0021 step 9); consumers
// (attack resolution, spell preparation, the XP ledger) depend on Lookup and
// BardXPThreshold only.
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

// XPRow is one row of PHB Table 25 (Thief/Bard column): the experience
// points that reach Level.
type XPRow struct {
	Level int
	XP    int
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
	xp   []int // xp[level-1] is the threshold of that level
	// classes are the class and race tables of a format-4 catalogue (ADR
	// 0021 step 10); nil for a format-3 one.
	classes *ClassTables
	// levelLimits are the racial level limits of a format-5 catalogue (ADR
	// 0021 step 11); nil for a format-3 or format-4 one.
	levelLimits *LevelLimits
}

var _ Lookup = (*Tables)(nil)

// New validates the rows and builds the lookup. Strength rows must cover
// scores 1..25 with 18 split into a contiguous 0..100 exceptional range;
// dexterity rows 1..25 contiguously; bard rows levels 1..20 exactly once;
// experience rows levels 1..20 in order, from 0 and strictly increasing.
func New(str []StrengthRow, dex []DexRow, bard []BardRow, xp []XPRow) (*Tables, error) {
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
	thresholds, err := validateXP(xp)
	if err != nil {
		return nil, fmt.Errorf("experience table: %w", err)
	}
	return &Tables{
		str:  append([]StrengthRow(nil), str...),
		dex:  append([]DexRow(nil), dex...),
		bard: byLevel,
		xp:   thresholds,
	}, nil
}

// validateXP checks that the rows are levels 1..MaxBardLevel in order, that
// level 1 starts at 0 XP and that every level needs more XP than the one
// before, and returns the thresholds indexed by level-1.
func validateXP(rows []XPRow) ([]int, error) {
	if len(rows) != MaxBardLevel {
		return nil, fmt.Errorf("needs levels 1..%d, got %d rows", MaxBardLevel, len(rows))
	}
	out := make([]int, 0, MaxBardLevel)
	prev := 0
	for i, r := range rows {
		if r.Level != i+1 {
			return nil, fmt.Errorf("row %d: level %d expected, got %d", i, i+1, r.Level)
		}
		if i == 0 && r.XP != 0 {
			return nil, errors.New("level 1 must start at 0 XP")
		}
		if i > 0 && r.XP <= prev {
			return nil, fmt.Errorf("level %d: %d XP is not above level %d", r.Level, r.XP, i)
		}
		out = append(out, r.XP)
		prev = r.XP
	}
	return out, nil
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

// WithClasses returns a copy of t carrying the class and race tables.
func (t *Tables) WithClasses(c *ClassTables) *Tables {
	out := *t
	out.classes = c
	return &out
}

// WithLevelLimits returns a copy of t carrying the racial level limits.
func (t *Tables) WithLevelLimits(l *LevelLimits) *Tables {
	out := *t
	out.levelLimits = l
	return &out
}

// LevelLimits returns the racial level limits (DMG Table 7, ADR 0021 step
// 11), false when the catalogue is format 3 or 4 and carries none.
func (t *Tables) LevelLimits() (*LevelLimits, bool) {
	return t.levelLimits, t.levelLimits != nil
}

// Classes returns the class and race tables (ADR 0030 step 23), false when
// the catalogue is format 3 and carries none.
func (t *Tables) Classes() (*ClassTables, bool) {
	return t.classes, t.classes != nil
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

// BardXPThreshold returns the experience points that reach level (PHB Table
// 25, Thief/Bard column), or an error outside 1..MaxBardLevel.
func (t *Tables) BardXPThreshold(level int) (int, error) {
	if level < 1 || level > len(t.xp) {
		return 0, fmt.Errorf("bard level %d out of range 1..%d", level, MaxBardLevel)
	}
	return t.xp[level-1], nil
}

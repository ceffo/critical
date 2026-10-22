package xpsvc

// ClassAmount is one active class entry's share of an award (ADR 0042 step
// 5.1).
type ClassAmount struct {
	Class  string
	Amount int
}

// Entry is one award in the ledger. Date is the day it was recorded
// (YYYY-MM-DD, server local time). BonusAmount and Total are computed (ADR
// 0015 step 1), never stored.
type Entry struct {
	ID          int64
	Date        string
	Session     string
	Reason      string
	Base        int
	Bonus       bool
	BonusAmount int
	Total       int
	// Split is the award's total shared between the active class entries of
	// a multi-class character (computed; nil for one class).
	Split []ClassAmount
}

// EntryInput is a new award.
type EntryInput struct {
	Base    int
	Bonus   bool
	Reason  string
	Session string
}

// EntryPatch changes the fields that are set.
type EntryPatch struct {
	Base    *int
	Bonus   *bool
	Reason  *string
	Session *string
}

// Amount is what one entry adds: its base, the bonus and their sum.
type Amount struct {
	Base        int
	BonusAmount int
	Total       int
	// Split is Total shared between the active class entries of a
	// multi-class character (ADR 0042 step 5.1); nil for one class.
	Split []ClassAmount
}

// CellStatus places a level cell against the sheet's level.
type CellStatus string

// Level cell statuses.
const (
	CellPast    CellStatus = "past"    // below the current level
	CellCurrent CellStatus = "current" // the sheet's level
	CellNext    CellStatus = "next"    // the level after it
	CellAhead   CellStatus = "ahead"   // further up
)

// LevelCell is one level of PHB Table 25 with the XP that reaches it.
type LevelCell struct {
	Level     int
	Threshold int
	Status    CellStatus
}

// Progress is where the total stands between the sheet's level and the
// next (ADR 0015 step 3). At the last level of the table NextLevel is 0,
// NextThreshold nil, ToNext 0 and the progress full.
type Progress struct {
	Level           int
	CurrentFloor    int
	NextLevel       int
	NextThreshold   *int
	ToNext          int
	Progress        float64 // 0..1
	ProgressPercent int     // Progress in whole percent, rounded down
	Levels          []LevelCell
}

// Ledger is the whole XP state: the sheet's starting XP, the total, the
// progress, whether the bonus applies by default and the entries, newest
// first.
type Ledger struct {
	Opening       int
	Total         int
	BonusEligible bool
	Progress
	Entries []Entry
}

package actionlog

import "time"

// HPEvent is one HP change as shown by the HP panel: an hp_change entry of the
// action log with its effective (amended) delta and source. Pure domain — no
// transport tags. Handlers map to a DTO before serializing.
type HPEvent struct {
	ID          int64
	CharacterID int64
	Delta       int
	Source      string
	CreatedAt   time.Time
}

// HPState is the materialized view of a character's hit points: current value,
// maximum, the floor and the condition (ADR 0011), and the last few ledger
// entries.
type HPState struct {
	CurrentHp int
	MaxHp     int
	// FloorHp is the lowest HP a character can have (HPFloor).
	FloorHp   int
	Condition Condition
	Events    []HPEvent
}

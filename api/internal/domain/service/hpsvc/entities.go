package hpsvc

import "time"

// HpEvent is a single change in the append-only HP ledger. Pure domain — no
// transport tags. Handlers map to a DTO before serializing.
type HpEvent struct {
	ID          int64
	CharacterID int64
	Delta       int
	Source      string
	CreatedAt   time.Time
}

// HPState is the materialized view of a character's hit points: current value,
// maximum, and the last few ledger entries.
type HPState struct {
	CurrentHp int
	MaxHp     int
	Events    []HpEvent
}

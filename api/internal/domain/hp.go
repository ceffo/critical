package domain

import (
	"database/sql"
	"fmt"
	"time"
)

type HpEvent struct {
	ID          int64
	CharacterID int64
	Delta       int
	Source      string
	CreatedAt   time.Time
}

type HPState struct {
	CurrentHp int       `json:"currentHp"`
	MaxHp     int       `json:"maxHp"`
	Events    []HpEvent `json:"events"`
}

// AppendEvent inserts a new HP event into the append-only ledger.
func AppendEvent(db *sql.DB, characterID int64, delta int, source string) error {
	_, err := db.Exec(
		`INSERT INTO hp_events (character_id, delta, source) VALUES (?, ?, ?)`,
		characterID, delta, source,
	)
	if err != nil {
		return fmt.Errorf("insert hp_event: %w", err)
	}
	return nil
}

// GetCurrentHP returns SUM(delta) for the character's ledger.
// Returns 0 when no events exist yet.
func GetCurrentHP(db *sql.DB, characterID int64) (int, error) {
	var sum sql.NullInt64
	if err := db.QueryRow(
		`SELECT SUM(delta) FROM hp_events WHERE character_id = ?`, characterID,
	).Scan(&sum); err != nil {
		return 0, fmt.Errorf("sum hp_events: %w", err)
	}
	return int(sum.Int64), nil
}

// GetHistory returns the most recent n events in descending order.
func GetHistory(db *sql.DB, characterID int64, limit int) ([]HpEvent, error) {
	rows, err := db.Query(
		`SELECT id, character_id, delta, source, created_at
		 FROM hp_events
		 WHERE character_id = ?
		 ORDER BY id DESC
		 LIMIT ?`,
		characterID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query hp_events: %w", err)
	}
	defer rows.Close()

	var events []HpEvent
	for rows.Next() {
		var e HpEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.CharacterID, &e.Delta, &e.Source, &createdAt); err != nil {
			return nil, fmt.Errorf("scan hp_event: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// UndoLastEvent inserts a correction row that negates the last event (append-only undo).
// No-op if there are no events.
func UndoLastEvent(db *sql.DB, characterID int64) error {
	var lastDelta int
	var lastSource string
	err := db.QueryRow(
		`SELECT delta, source FROM hp_events WHERE character_id = ? ORDER BY id DESC LIMIT 1`,
		characterID,
	).Scan(&lastDelta, &lastSource)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query last hp_event: %w", err)
	}
	return AppendEvent(db, characterID, -lastDelta, "undo:"+lastSource)
}

// GetHPState returns current HP (max + ledger sum), max HP, and recent events.
func GetHPState(db *sql.DB, characterID int64) (*HPState, error) {
	var maxHP int
	if err := db.QueryRow(
		`SELECT hp_max FROM characters WHERE id = ?`, characterID,
	).Scan(&maxHP); err != nil {
		return nil, fmt.Errorf("get hp_max: %w", err)
	}

	ledgerSum, err := GetCurrentHP(db, characterID)
	if err != nil {
		return nil, err
	}

	events, err := GetHistory(db, characterID, 5)
	if err != nil {
		return nil, err
	}

	return &HPState{
		CurrentHp: maxHP + ledgerSum,
		MaxHp:     maxHP,
		Events:    events,
	}, nil
}

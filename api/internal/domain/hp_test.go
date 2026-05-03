package domain_test

import (
	"database/sql"
	"testing"

	critdb "github.com/ceffo/critical/api/internal/db"
	"github.com/ceffo/critical/api/internal/domain"
)

const testSeedsDir = "../../../seeds"

func openTestDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	database, err := critdb.Open(":memory:", testSeedsDir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	var charID int64
	if err := database.QueryRow("SELECT id FROM characters LIMIT 1").Scan(&charID); err != nil {
		t.Fatalf("get character id: %v", err)
	}
	return database, charID
}

func TestAppendAndGetCurrentHP(t *testing.T) {
	db, charID := openTestDB(t)

	var maxHP int
	db.QueryRow("SELECT hp_max FROM characters WHERE id=?", charID).Scan(&maxHP)

	// Current HP starts at max (no events yet → SUM = 0 but GetCurrentHP returns max + SUM)
	state, err := domain.GetHPState(db, charID)
	if err != nil {
		t.Fatalf("GetHPState: %v", err)
	}
	if state.CurrentHp != state.MaxHp {
		t.Errorf("fresh character: currentHP=%d want %d (maxHP)", state.CurrentHp, state.MaxHp)
	}

	// Take 10 damage
	if err := domain.AppendEvent(db, charID, -10, "sword"); err != nil {
		t.Fatalf("AppendEvent damage: %v", err)
	}

	state, err = domain.GetHPState(db, charID)
	if err != nil {
		t.Fatalf("GetHPState after damage: %v", err)
	}
	want := maxHP - 10
	if state.CurrentHp != want {
		t.Errorf("after -10: currentHP=%d want %d", state.CurrentHp, want)
	}

	// Heal 5
	if err := domain.AppendEvent(db, charID, 5, "cure light wounds"); err != nil {
		t.Fatalf("AppendEvent heal: %v", err)
	}

	state, err = domain.GetHPState(db, charID)
	if err != nil {
		t.Fatalf("GetHPState after heal: %v", err)
	}
	want = maxHP - 5
	if state.CurrentHp != want {
		t.Errorf("after -10+5: currentHP=%d want %d", state.CurrentHp, want)
	}
}

func TestGetHistory(t *testing.T) {
	db, charID := openTestDB(t)

	events := []struct {
		delta  int
		source string
	}{
		{-5, "arrow"},
		{-10, "sword"},
		{3, "bandage"},
		{-7, "fire"},
		{-2, "fall"},
		{10, "cure"},
	}
	for _, e := range events {
		if err := domain.AppendEvent(db, charID, e.delta, e.source); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	history, err := domain.GetHistory(db, charID, 5)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("GetHistory limit=5: got %d events", len(history))
	}
	// Most recent first: "cure" should be first
	if history[0].Source != "cure" {
		t.Errorf("first event source=%q want %q", history[0].Source, "cure")
	}
}

func TestUndoLastEvent(t *testing.T) {
	db, charID := openTestDB(t)

	var maxHP int
	db.QueryRow("SELECT hp_max FROM characters WHERE id=?", charID).Scan(&maxHP)

	if err := domain.AppendEvent(db, charID, -20, "giant"); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := domain.UndoLastEvent(db, charID); err != nil {
		t.Fatalf("UndoLastEvent: %v", err)
	}

	state, err := domain.GetHPState(db, charID)
	if err != nil {
		t.Fatalf("GetHPState after undo: %v", err)
	}
	if state.CurrentHp != maxHP {
		t.Errorf("after undo: currentHP=%d want %d (maxHP)", state.CurrentHp, maxHP)
	}

	// Undo with no events should not error
	db2, charID2 := openTestDB(t)
	if err := domain.UndoLastEvent(db2, charID2); err != nil {
		t.Errorf("UndoLastEvent on empty ledger: %v", err)
	}
}

func TestUndoIsAppendNotDelete(t *testing.T) {
	db, charID := openTestDB(t)

	if err := domain.AppendEvent(db, charID, -20, "giant"); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := domain.UndoLastEvent(db, charID); err != nil {
		t.Fatalf("UndoLastEvent: %v", err)
	}

	// Should have 2 rows: the original -20 and the correction +20
	var count int
	db.QueryRow("SELECT COUNT(*) FROM hp_events WHERE character_id=?", charID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 rows after undo (append-only), got %d", count)
	}
}

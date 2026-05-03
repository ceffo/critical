package db_test

import (
	"database/sql"
	"testing"

	critdb "github.com/ceffo/critical/api/internal/db"
	_ "modernc.org/sqlite"
)

const testSeedsDir = "../../../seeds"

func TestOpenAndMigrate(t *testing.T) {
	database, err := critdb.Open(":memory:", testSeedsDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("error closing database: %v", err)
		}
	}()

	tables := []string{
		"characters",
		"hp_events",
		"xp_events",
		"active_buffs",
		"spell_slots",
		"round_state",
	}
	for _, table := range tables {
		var name string
		err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %q not created by migrations", table)
		} else if err != nil {
			t.Errorf("querying table %q: %v", table, err)
		}
	}
}

func TestSeedCharacter(t *testing.T) {
	database, err := critdb.Open(":memory:", testSeedsDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("error closing database: %v", err)
		}
	}()

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM characters").Scan(&count); err != nil {
		t.Fatalf("count characters: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 seeded character, got %d", count)
	}
}

func TestSeedIdempotent(t *testing.T) {
	database, err := critdb.Open(":memory:", testSeedsDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("error closing database: %v", err)
		}
	}()

	// Second open on same DB won't run seed again because table is not empty.
	// Simulate by calling seed again and confirming count stays 1.
	if err := critdb.SeedCharacter(database, testSeedsDir+"/character.json"); err != nil {
		t.Fatalf("SeedCharacter second call: %v", err)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM characters").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 character after idempotent seed, got %d", count)
	}
}

package domain

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
)

func setupSpellTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE characters (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE spell_slots (id INTEGER PRIMARY KEY, character_id INTEGER, slots TEXT);
		INSERT INTO characters (id, name) VALUES (1, 'Bard');
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestSpellSlotPersistence(t *testing.T) {
	db := setupSpellTestDB(t)
	defer db.Close()

	charID := int64(1)
	slots := []SpellSlot{
		{Level: 1, PreparedSpellName: "Armor", Used: false},
		{Level: 2, PreparedSpellName: "Web", Used: true},
	}

	err := SaveSpellSlots(db, charID, slots)
	assert.NoError(t, err)

	loaded, err := GetSpellSlots(db, charID)
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, "Armor", loaded[0].PreparedSpellName)
	assert.False(t, loaded[0].Used)
	assert.True(t, loaded[1].Used)
}

func TestCastSpell(t *testing.T) {
	slots := []SpellSlot{
		{Level: 1, PreparedSpellName: "Armor", Used: false},
	}

	// Successful cast
	newSlots, err := CastSpell(slots, 0)
	assert.NoError(t, err)
	assert.True(t, newSlots[0].Used)
	assert.False(t, slots[0].Used) // Ensure original slice not mutated

	// Double cast
	_, err = CastSpell(newSlots, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already used")

	// Invalid index
	_, err = CastSpell(slots, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid slot index")
}

func TestResetSpellSlots(t *testing.T) {
	slots := []SpellSlot{
		{Level: 1, PreparedSpellName: "Armor", Used: true},
		{Level: 2, PreparedSpellName: "Web", Used: true},
	}

	newSlots := ResetSpellSlots(slots)
	assert.Len(t, newSlots, 2)
	assert.False(t, newSlots[0].Used)
	assert.False(t, newSlots[1].Used)
	assert.True(t, slots[0].Used) // Ensure original slice not mutated
}

package domain

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE characters (id INTEGER PRIMARY KEY, name TEXT, str INTEGER, thac0_base INTEGER, hp_max INTEGER);
		CREATE TABLE active_buffs (id INTEGER PRIMARY KEY, character_id INTEGER, buffs TEXT);
		CREATE TABLE round_state (id INTEGER PRIMARY KEY, character_id INTEGER, round_number INTEGER);
		INSERT INTO characters (id, name, str, thac0_base, hp_max) VALUES (1, 'Bard', 18, 16, 20);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestBuffPersistence(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	charID := int64(1)
	wb := 1
	buffs := []BuffEffect{
		{ID: "1", Name: "Bless", Type: "duration", Duration: 10, Effects: BuffEffects{WeaponBuff: &wb}},
	}

	err := SaveActiveBuffs(db, charID, buffs)
	assert.NoError(t, err)

	loaded, err := GetActiveBuffs(db, charID)
	assert.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "Bless", loaded[0].Name)
}

func TestRoundPersistence(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	charID := int64(1)
	err := SaveRoundNumber(db, charID, 5)
	assert.NoError(t, err)

	round, err := GetRoundNumber(db, charID)
	assert.NoError(t, err)
	assert.Equal(t, 5, round)
}

func TestAdvanceRound(t *testing.T) {
	buffs := []BuffEffect{
		{ID: "dur-1", Name: "Bless", Type: "duration", Duration: 2},
		{ID: "dur-2", Name: "Shield", Type: "duration", Duration: 1},
		{ID: "chg-1", Name: "Power", Type: "charge", Charges: 5},
	}

	active, expired := AdvanceRound(buffs)
	assert.Len(t, active, 2)
	assert.Len(t, expired, 1)
	assert.Equal(t, "dur-1", active[0].ID)
	assert.Equal(t, 1, active[0].Duration)
	assert.Equal(t, "dur-2", expired[0].ID)
	assert.Equal(t, "chg-1", active[1].ID)
	assert.Equal(t, 5, active[1].Charges) // Charges should not decrement on round advance
}

func TestTriggerChargeBuff(t *testing.T) {
	buffs := []BuffEffect{
		{ID: "chg-1", Name: "Shield", Type: "charge", Charges: 2},
		{ID: "chg-2", Name: "Strike", Type: "charge", Charges: 1},
	}

	// Trigger 1st buff
	buffs = TriggerChargeBuff(buffs, "chg-1")
	assert.Len(t, buffs, 2)
	assert.Equal(t, 1, buffs[0].Charges)

	// Trigger 2nd buff (should be removed)
	buffs = TriggerChargeBuff(buffs, "chg-2")
	assert.Len(t, buffs, 1)
	assert.Equal(t, "chg-1", buffs[0].ID)

	// Trigger 1st buff again (should be removed)
	buffs = TriggerChargeBuff(buffs, "chg-1")
	assert.Len(t, buffs, 0)
}


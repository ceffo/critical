// Package db handles SQLite connection, sequential migrations, and seed loading.
//
// seeds/character.json schema:
//
//	{
//	  "name":       string,   // character name
//	  "str":        int,      // Strength (3–18)
//	  "dex":        int,      // Dexterity (3–18)
//	  "con":        int,      // Constitution (3–18)
//	  "int":        int,      // Intelligence (3–18)
//	  "wis":        int,      // Wisdom (3–18)
//	  "cha":        int,      // Charisma (3–18)
//	  "hp_max":     int,      // maximum hit points
//	  "thac0_base": int,      // base THAC0 before modifiers (lower is better)
//	  "level":      int,      // character level
//	  "xp_total":   int       // total experience points
//	}
package db

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open opens (or creates) the SQLite database at dsn, runs all migrations in
// order, and seeds the characters table from <seedsDir>/character.json if it
// is empty. Pass ":memory:" for dsn in tests; pass the repo-root seeds/ path
// in production (e.g. "seeds").
func Open(dsn, seedsDir string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	if err := SeedCharacter(db, seedsDir+"/character.json"); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed character: %w", err)
	}

	if err := SeedSpells(db, seedsDir+"/spells.json"); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed spells: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		content, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec %s: %w", e.Name(), err)
		}
	}
	return nil
}

type characterSeed struct {
	Name             string `json:"name"`
	Str              int    `json:"str"`
	Dex              int    `json:"dex"`
	Con              int    `json:"con"`
	Int              int    `json:"int"`
	Wis              int    `json:"wis"`
	Cha              int    `json:"cha"`
	HPMax            int    `json:"hp_max"`
	Thac0Base        int    `json:"thac0_base"`
	Level            int    `json:"level"`
	XPTotal          int    `json:"xp_total"`
	SaveParaPoison   int    `json:"save_para_poison"`
	SaveRodStaffWand int    `json:"save_rod_staff_wand"`
	SavePetriPoly    int    `json:"save_petri_poly"`
	SaveBreath       int    `json:"save_breath"`
	SaveSpell        int    `json:"save_spell"`
	Slots            []interface{} `json:"slots"`
}

// SeedCharacter inserts the character from seedPath if the characters table is
// empty. Safe to call multiple times — it is a no-op when a row already exists.
func SeedCharacter(db *sql.DB, seedPath string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM characters").Scan(&count); err != nil {
		return fmt.Errorf("count characters: %w", err)
	}
	if count > 0 {
		return nil
	}

	data, err := os.ReadFile(seedPath)
	if err != nil {
		// Missing seed file is not fatal — skip seeding silently.
		return nil
	}

	var c characterSeed
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("parse character.json: %w", err)
	}
	if c.Name == "" {
		// Empty/stub seed file — skip.
		return nil
	}

	res, err := db.Exec(
		`INSERT INTO characters (
			name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
			save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Str, c.Dex, c.Con, c.Int, c.Wis, c.Cha,
		c.HPMax, c.Thac0Base, c.Level, c.XPTotal,
		c.SaveParaPoison, c.SaveRodStaffWand, c.SavePetriPoly, c.SaveBreath, c.SaveSpell,
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}

	charID, _ := res.LastInsertId()

	// Initialize state tables
	slotsJSON, _ := json.Marshal(c.Slots)
	if len(c.Slots) == 0 {
		slotsJSON = []byte("[]")
	}
	db.Exec("INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)", charID, string(slotsJSON))
	db.Exec("INSERT INTO active_buffs (character_id, buffs) VALUES (?, '[]')", charID)
	db.Exec("INSERT INTO round_state (character_id, round_number) VALUES (?, 0)", charID)

	return nil
}

type spellData struct {
	Name         string `json:"name"`
	Class        string `json:"class"`
	Level        int    `json:"level"`
	School       string `json:"school"`
	Sphere       string `json:"sphere"`
	CastingTime  string `json:"casting_time"`
	Range        string `json:"range"`
	Duration     string `json:"duration"`
	AreaOfEffect string `json:"area_of_effect"`
	Components   string `json:"components"`
	SavingThrow  string `json:"saving_throw"`
	Description  string `json:"description"`
}

func SeedSpells(db *sql.DB, seedPath string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM spells").Scan(&count); err != nil {
		return fmt.Errorf("count spells: %w", err)
	}
	if count > 0 {
		return nil
	}

	data, err := os.ReadFile(seedPath)
	if err != nil {
		return nil
	}

	var spells []spellData
	if err := json.Unmarshal(data, &spells); err != nil {
		return fmt.Errorf("parse spells.json: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO spells (
			name, class, level, school, sphere, casting_time, range, 
			duration, area_of_effect, components, saving_throw, description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range spells {
		_, err := stmt.Exec(
			s.Name, s.Class, s.Level, s.School, s.Sphere, s.CastingTime, s.Range,
			s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow, s.Description,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}


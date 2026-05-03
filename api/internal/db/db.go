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
	Name      string `json:"name"`
	Str       int    `json:"str"`
	Dex       int    `json:"dex"`
	Con       int    `json:"con"`
	Int       int    `json:"int"`
	Wis       int    `json:"wis"`
	Cha       int    `json:"cha"`
	HPMax     int    `json:"hp_max"`
	Thac0Base int    `json:"thac0_base"`
	Level     int    `json:"level"`
	XPTotal   int    `json:"xp_total"`
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

	_, err = db.Exec(
		`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Str, c.Dex, c.Con, c.Int, c.Wis, c.Cha,
		c.HPMax, c.Thac0Base, c.Level, c.XPTotal,
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	return nil
}

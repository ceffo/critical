package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

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
	Slots            []any  `json:"slots"`
}

type spellSeed struct {
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

// Seed loads character.json and spells.json from seedsDir if their tables are
// empty. Missing seed files are silently ignored.
func (d *DB) Seed(ctx context.Context, seedsDir string) error {
	if err := d.seedCharacter(ctx, filepath.Join(filepath.Clean(seedsDir), "character.json")); err != nil {
		return fmt.Errorf("seed character: %w", err)
	}
	if err := d.seedSpells(ctx, filepath.Join(filepath.Clean(seedsDir), "spells.json")); err != nil {
		return fmt.Errorf("seed spells: %w", err)
	}
	return nil
}

func (d *DB) seedCharacter(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM characters").Scan(&count); err != nil {
		return fmt.Errorf("count characters: %w", err)
	}
	if count > 0 {
		return nil
	}

	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return nil
	}

	var c characterSeed
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("parse character.json: %w", err)
	}
	if c.Name == "" {
		return nil
	}

	res, err := d.ExecContext(ctx,
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

	slotsJSON, err := json.Marshal(c.Slots)
	if err != nil {
		return fmt.Errorf("marshal slots: %w", err)
	}
	if len(c.Slots) == 0 {
		slotsJSON = []byte("[]")
	}
	if _, err := d.ExecContext(ctx, "INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)", charID, string(slotsJSON)); err != nil {
		return fmt.Errorf("insert spell_slots: %w", err)
	}
	if _, err := d.ExecContext(ctx, "INSERT INTO active_buffs (character_id, buffs) VALUES (?, '[]')", charID); err != nil {
		return fmt.Errorf("insert active_buffs: %w", err)
	}
	if _, err := d.ExecContext(ctx, "INSERT INTO round_state (character_id, round_number) VALUES (?, 0)", charID); err != nil {
		return fmt.Errorf("insert round_state: %w", err)
	}
	return nil
}

func (d *DB) seedSpells(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM spells").Scan(&count); err != nil {
		return fmt.Errorf("count spells: %w", err)
	}
	if count > 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return nil
	}
	var spells []spellSeed
	if err := json.Unmarshal(data, &spells); err != nil {
		return fmt.Errorf("parse spells.json: %w", err)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Default().Warn("seed spells rollback", "error", rbErr)
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO spells (
			name, class, level, school, sphere, casting_time, range,
			duration, area_of_effect, components, saving_throw, description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, s := range spells {
		if _, err := stmt.ExecContext(ctx,
			s.Name, s.Class, s.Level, s.School, s.Sphere, s.CastingTime, s.Range,
			s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow, s.Description,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

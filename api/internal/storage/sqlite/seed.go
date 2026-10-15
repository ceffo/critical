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

	"github.com/ceffo/critical/api/internal/domain/spellstats"
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

type weaponSeed struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DamageSM    string `json:"damage_sm"`
	DamageL     string `json:"damage_l"`
	MagicToHit  int    `json:"magic_to_hit"`
	MagicDamage int    `json:"magic_damage"`
	Kind        string `json:"kind"`
	TwoHanded   bool   `json:"two_handed"`
	AmmoKind    string `json:"ammo_kind"`
}

// itemSeed is one seeds/items.json entry: behaviour is
// stock, charges or daily, type a free-text label, max_count defaults to
// count (a daily item's refill value).
type itemSeed struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Behaviour  string `json:"behaviour"`
	MagicBonus int    `json:"magic_bonus"`
	AmmoKind   string `json:"ammo_kind"`
	Count      int    `json:"count"`
	MaxCount   *int   `json:"max_count"`
	EffectDice string `json:"effect_dice"`
	EffectSave string `json:"effect_save"`
	Notes      string `json:"notes"`
}

// Seed loads character.json, spells.json, weapons.json, items.json and
// skills.json from seedsDir into their tables: character, weapons, items and
// skills only if their table is empty, spells reconciled by (name, class).
// Missing character and spell files are ignored. A missing weapons, items or
// skills file while its table is empty is an error: the catalog would be
// empty.
func (d *DB) Seed(ctx context.Context, seedsDir string) error {
	if err := d.seedCharacter(ctx, filepath.Join(filepath.Clean(seedsDir), "character.json")); err != nil {
		return fmt.Errorf("seed character: %w", err)
	}
	if err := d.seedSpells(ctx, filepath.Join(filepath.Clean(seedsDir), "spells.json")); err != nil {
		return fmt.Errorf("seed spells: %w", err)
	}
	if err := d.seedWeapons(ctx, filepath.Join(filepath.Clean(seedsDir), "weapons.json")); err != nil {
		return fmt.Errorf("seed weapons: %w", err)
	}
	if err := d.seedItems(ctx, filepath.Join(filepath.Clean(seedsDir), "items.json")); err != nil {
		return fmt.Errorf("seed items: %w", err)
	}
	if err := d.seedSkills(ctx, filepath.Join(filepath.Clean(seedsDir), "skills.json")); err != nil {
		return fmt.Errorf("seed skills: %w", err)
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

// seedSpells reconciles the spells table with spells.json: rows are keyed by
// (name, class, level) (two Priest "Fortify" spells differ by level).
// Missing ones are inserted, existing ones get the seed's values and typed
// stats, nothing is ever deleted and ids never change. It runs in one
// transaction and a second run changes nothing. A missing seed file leaves
// the table alone (only untyped rows get their stats).
func (d *DB) seedSpells(ctx context.Context, seedPath string) error {
	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return d.backfillSpellStats(ctx)
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

	// A database may hold a (name, class, level) twice (an older, uncleaned
	// catalog): only the oldest row is kept in step with the seed, the others
	// are left alone.
	update, err := tx.PrepareContext(ctx, `
		UPDATE spells SET school = ?, sphere = ?, casting_time = ?, range = ?,
			duration = ?, area_of_effect = ?, components = ?, saving_throw = ?, description = ?, stats = ?
		WHERE id = (SELECT MIN(id) FROM spells WHERE name = ? AND class = ? AND level = ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = update.Close() }()
	insert, err := tx.PrepareContext(ctx, `
		INSERT INTO spells (
			name, class, level, school, sphere, casting_time, range,
			duration, area_of_effect, components, saving_throw, description, stats
		) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM spells WHERE name = ? AND class = ? AND level = ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = insert.Close() }()

	for _, s := range spells {
		stats, err := spellStatsJSON(s.CastingTime, s.Range, s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow)
		if err != nil {
			return fmt.Errorf("spell %q: %w", s.Name, err)
		}
		if _, err := update.ExecContext(ctx,
			s.School, s.Sphere, s.CastingTime, s.Range, s.Duration, s.AreaOfEffect,
			s.Components, s.SavingThrow, s.Description, stats, s.Name, s.Class, s.Level,
		); err != nil {
			return fmt.Errorf("update spell %q: %w", s.Name, err)
		}
		if _, err := insert.ExecContext(ctx,
			s.Name, s.Class, s.Level, s.School, s.Sphere, s.CastingTime, s.Range,
			s.Duration, s.AreaOfEffect, s.Components, s.SavingThrow, s.Description, stats,
			s.Name, s.Class, s.Level,
		); err != nil {
			return fmt.Errorf("insert spell %q: %w", s.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.backfillSpellStats(ctx)
}

// spellStatsJSON parses the raw PHB stat texts into the typed stats JSON.
func spellStatsJSON(castingTime, rng, duration, area, components, save string) (string, error) {
	b, err := json.Marshal(spellstats.Parse(spellstats.Raw{
		CastingTime: castingTime, Range: rng, Duration: duration,
		AreaOfEffect: area, Components: components, SavingThrow: save,
	}))
	if err != nil {
		return "", fmt.Errorf("marshal spell stats: %w", err)
	}
	return string(b), nil
}

// backfillSpellStats types the spells that predate the stats column (a
// database seeded before migration 009). Cheap and a no-op once filled.
func (d *DB) backfillSpellStats(ctx context.Context) error {
	type row struct {
		id                                                   int64
		castingTime, rng, duration, area, components, saving string
	}
	rows, err := d.QueryContext(ctx, `SELECT id, COALESCE(casting_time,''), COALESCE(range,''), COALESCE(duration,''),
		COALESCE(area_of_effect,''), COALESCE(components,''), COALESCE(saving_throw,'') FROM spells WHERE stats IS NULL`)
	if err != nil {
		return fmt.Errorf("select spells without stats: %w", err)
	}
	var todo []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.castingTime, &r.rng, &r.duration, &r.area, &r.components, &r.saving); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan spell: %w", err)
		}
		todo = append(todo, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(todo) == 0 {
		return nil
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for _, r := range todo {
			stats, err := spellStatsJSON(r.castingTime, r.rng, r.duration, r.area, r.components, r.saving)
			if err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, "UPDATE spells SET stats = ? WHERE id = ?", stats, r.id); err != nil {
				return fmt.Errorf("update spell stats: %w", err)
			}
		}
		return nil
	})
}

func (d *DB) seedWeapons(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM weapons").Scan(&count); err != nil {
		return fmt.Errorf("count weapons: %w", err)
	}
	if count > 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return fmt.Errorf("weapons table is empty and its seed file is unreadable (the catalog would be empty): %w", err)
	}
	var weapons []weaponSeed
	if err := json.Unmarshal(data, &weapons); err != nil {
		return fmt.Errorf("parse weapons.json: %w", err)
	}
	seen := make(map[string]bool, len(weapons))
	for i, w := range weapons {
		if seen[w.ID] {
			return fmt.Errorf("weapons.json entry %d: duplicate id %q", i, w.ID)
		}
		seen[w.ID] = true
		if w.ID == "" || w.Name == "" || w.DamageSM == "" || w.DamageL == "" {
			return fmt.Errorf("weapons.json entry %d: id, name, damage_sm and damage_l are required", i)
		}
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for i, w := range weapons {
			if _, err := q.ExecContext(ctx, `
				INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				w.ID, i, w.Name, w.DamageSM, w.DamageL, w.MagicToHit, w.MagicDamage, w.Kind, w.TwoHanded, w.AmmoKind,
			); err != nil {
				return fmt.Errorf("insert weapon %q: %w", w.ID, err)
			}
		}
		return nil
	})
}

func (d *DB) seedItems(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		return fmt.Errorf("count items: %w", err)
	}
	if count > 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return fmt.Errorf("items table is empty and its seed file is unreadable (the catalog would be empty): %w", err)
	}
	var items []itemSeed
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parse items.json: %w", err)
	}
	seen := make(map[string]bool, len(items))
	for i, it := range items {
		if it.ID == "" || it.Name == "" {
			return fmt.Errorf("items.json entry %d: id and name are required", i)
		}
		if seen[it.ID] {
			return fmt.Errorf("items.json entry %d: duplicate id %q", i, it.ID)
		}
		seen[it.ID] = true
		switch it.Behaviour {
		case "stock", "charges", "daily":
		default:
			return fmt.Errorf("items.json entry %d (%q): behaviour must be stock, charges or daily, got %q", i, it.ID, it.Behaviour)
		}
		if it.Count < 0 {
			return fmt.Errorf("items.json entry %d (%q): negative count", i, it.ID)
		}
		if it.MaxCount != nil && it.Count > *it.MaxCount {
			return fmt.Errorf("items.json entry %d (%q): count %d exceeds max_count %d", i, it.ID, it.Count, *it.MaxCount)
		}
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for i, it := range items {
			maxCount := it.Count
			if it.MaxCount != nil {
				maxCount = *it.MaxCount
			}
			if _, err := q.ExecContext(ctx, `
				INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				it.ID, i, it.Name, it.Type, it.Behaviour, it.MagicBonus, it.AmmoKind, it.Count, maxCount, it.EffectDice, it.EffectSave, it.Notes,
			); err != nil {
				return fmt.Errorf("insert item %q: %w", it.ID, err)
			}
		}
		return nil
	})
}

// skillsSeed is seeds/skills.json: the non-weapon proficiencies (ability key
// and modifier) and the bard skills (percentage) of ADR 0014. Ids are shared
// by both lists (a check is rolled by id) and named by the UI.
type skillsSeed struct {
	Proficiencies []struct {
		ID       string `json:"id"`
		Ability  string `json:"ability"`
		Modifier int    `json:"modifier"`
	} `json:"proficiencies"`
	BardSkills []struct {
		ID      string `json:"id"`
		Percent int    `json:"percent"`
	} `json:"bard_skills"`
}

var abilityKeys = map[string]bool{"str": true, "dex": true, "con": true, "int": true, "wis": true, "cha": true}

func (s skillsSeed) validate() error {
	seen := map[string]bool{}
	id := func(where string, i int, id string) error {
		if id == "" {
			return fmt.Errorf("skills.json %s %d: id is required", where, i)
		}
		if seen[id] {
			return fmt.Errorf("skills.json %s %d: duplicate id %q", where, i, id)
		}
		seen[id] = true
		return nil
	}
	for i, p := range s.Proficiencies {
		if err := id("proficiency", i, p.ID); err != nil {
			return err
		}
		if !abilityKeys[p.Ability] {
			return fmt.Errorf("skills.json proficiency %q: unknown ability %q", p.ID, p.Ability)
		}
	}
	for i, b := range s.BardSkills {
		if err := id("bard skill", i, b.ID); err != nil {
			return err
		}
		if b.Percent < 0 || b.Percent > 100 {
			return fmt.Errorf("skills.json bard skill %q: percent %d is outside 0-100", b.ID, b.Percent)
		}
	}
	return nil
}

// seedSkills fills the proficiencies and bard_skills tables from skills.json
// when both are empty, in one transaction. Rows already there are never
// touched, so a second run (a restart) changes nothing.
func (d *DB) seedSkills(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM proficiencies) + (SELECT COUNT(*) FROM bard_skills)").Scan(&count); err != nil {
		return fmt.Errorf("count skills: %w", err)
	}
	if count > 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Clean(seedPath))
	if err != nil {
		return fmt.Errorf("skill tables are empty and their seed file is unreadable: %w", err)
	}
	var seed skillsSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("parse skills.json: %w", err)
	}
	if err := seed.validate(); err != nil {
		return err
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for i, p := range seed.Proficiencies {
			if _, err := q.ExecContext(ctx,
				"INSERT INTO proficiencies (id, position, ability, modifier) VALUES (?, ?, ?, ?)",
				p.ID, i, p.Ability, p.Modifier,
			); err != nil {
				return fmt.Errorf("insert proficiency %q: %w", p.ID, err)
			}
		}
		for i, b := range seed.BardSkills {
			if _, err := q.ExecContext(ctx,
				"INSERT INTO bard_skills (id, position, percent) VALUES (?, ?, ?)",
				b.ID, i, b.Percent,
			); err != nil {
				return fmt.Errorf("insert bard skill %q: %w", b.ID, err)
			}
		}
		return nil
	})
}

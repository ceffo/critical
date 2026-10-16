package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// Seed loads character.json, weapons.json, items.json and skills.json from
// seedsDir into their tables: character, weapons and items only if their
// table is empty (once filled, only the names of weapons and items are
// reconciled, reconcileName), skills reconciled by id. A missing character
// file is ignored. A missing weapons, items or skills file while its table
// is empty is an error: the catalog would be empty. Spells are not seeds: they come
// from the catalogue artifact (ImportCatalogue).
func (d *DB) Seed(ctx context.Context, seedsDir string) error {
	if err := d.seedCharacter(ctx, seedfile.Path(seedsDir, seedfile.CharacterFile)); err != nil {
		return fmt.Errorf("seed character: %w", err)
	}
	if err := d.seedWeapons(ctx, seedfile.Path(seedsDir, seedfile.WeaponsFile)); err != nil {
		return fmt.Errorf("seed weapons: %w", err)
	}
	if err := d.seedItems(ctx, seedfile.Path(seedsDir, seedfile.ItemsFile)); err != nil {
		return fmt.Errorf("seed items: %w", err)
	}
	if err := d.seedSkills(ctx, seedfile.Path(seedsDir, seedfile.SkillsFile)); err != nil {
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

	c, ok, err := seedfile.ReadCharacter(seedPath)
	if err != nil || !ok {
		return err
	}

	languages, err := json.Marshal(append([]string{}, c.Languages...))
	if err != nil {
		return fmt.Errorf("marshal languages: %w", err)
	}
	res, err := d.ExecContext(ctx,
		`INSERT INTO characters (
			name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
			save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell, languages
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Str, c.Dex, c.Con, c.Int, c.Wis, c.Cha,
		c.HPMax, c.Thac0Base, c.Level, c.XPTotal,
		c.SaveParaPoison, c.SaveRodStaffWand, c.SavePetriPoly, c.SaveBreath, c.SaveSpell, string(languages),
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	charID, _ := res.LastInsertId()

	slotsJSON, err := c.SlotsJSON()
	if err != nil {
		return err
	}
	if _, err := d.ExecContext(ctx, "INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)", charID, slotsJSON); err != nil {
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

func (d *DB) seedWeapons(ctx context.Context, seedPath string) error {
	var count int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM weapons").Scan(&count); err != nil {
		return fmt.Errorf("count weapons: %w", err)
	}
	weapons, err := seedfile.ReadWeapons(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		if count > 0 {
			return nil
		}
		return fmt.Errorf("weapons table is empty and its seed file is unreadable (the catalog would be empty): %w", err)
	}
	if err != nil {
		return err
	}
	return d.WrapTx(ctx, func(q Querier) error {
		if count > 0 {
			for _, w := range weapons {
				if err := reconcileName(ctx, q, weaponName, w.ID, w.Name.Trimmed()); err != nil {
					return fmt.Errorf("weapon %q: %w", w.ID, err)
				}
			}
			return nil
		}
		for i, w := range weapons {
			key, err := addSeedText(ctx, q, w.Name.Trimmed())
			if err != nil {
				return fmt.Errorf("weapon %q: %w", w.ID, err)
			}
			if err := recordOffers(ctx, q, weaponName, w.ID, w.Name.Trimmed().Languages()); err != nil {
				return fmt.Errorf("weapon %q: %w", w.ID, err)
			}
			if _, err := q.ExecContext(ctx, `
				INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				w.ID, i, key, w.DamageSM, w.DamageL, w.MagicToHit, w.MagicDamage, w.Kind, w.TwoHanded, w.AmmoKind,
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
	items, err := seedfile.ReadItems(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		if count > 0 {
			return nil
		}
		return fmt.Errorf("items table is empty and its seed file is unreadable (the catalog would be empty): %w", err)
	}
	if err != nil {
		return err
	}
	return d.WrapTx(ctx, func(q Querier) error {
		if count > 0 {
			for _, it := range items {
				if err := reconcileName(ctx, q, itemName, it.ID, it.Name.Trimmed()); err != nil {
					return fmt.Errorf("item %q: %w", it.ID, err)
				}
			}
			return nil
		}
		for i, it := range items {
			key, err := addSeedText(ctx, q, it.Name.Trimmed())
			if err != nil {
				return fmt.Errorf("item %q: %w", it.ID, err)
			}
			if err := recordOffers(ctx, q, itemName, it.ID, it.Name.Trimmed().Languages()); err != nil {
				return fmt.Errorf("item %q: %w", it.ID, err)
			}
			if _, err := q.ExecContext(ctx, `
				INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				it.ID, i, key, it.Type, it.Behaviour, it.MagicBonus, it.AmmoKind, it.Count, it.Max(), it.EffectDice, it.EffectSave, it.Notes, it.PowersJSON,
			); err != nil {
				return fmt.Errorf("insert item %q: %w", it.ID, err)
			}
		}
		return nil
	})
}

// seededName says where a weapon's or an item's name key is read and the
// kind its seed offers are recorded under (seeded_name_languages).
type seededName struct {
	kind     string
	keyQuery string
}

var (
	weaponName = seededName{kind: "weapon", keyQuery: `SELECT name FROM weapons WHERE id = ?`}
	itemName   = seededName{kind: "item", keyQuery: `SELECT name FROM items WHERE id = ?`}
)

// addSeedText stores t under a new key, one more than the highest stored (as
// every writer of localized_texts), one row per language.
func addSeedText(ctx context.Context, q Querier, t localized.Text) (int64, error) {
	var key int64
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(MAX(text_id), 0) + 1 FROM localized_texts`).Scan(&key); err != nil {
		return 0, fmt.Errorf("next text key: %w", err)
	}
	for _, lang := range t.Languages() {
		if err := insertText(ctx, q, key, lang, t[lang]); err != nil {
			return 0, err
		}
	}
	return key, nil
}

func insertText(ctx context.Context, q Querier, key int64, lang, body string) error {
	if _, err := q.ExecContext(ctx, `INSERT INTO localized_texts (text_id, lang, body) VALUES (?, ?, ?)`, key, lang, body); err != nil {
		return fmt.Errorf("insert text %d %s: %w", key, lang, err)
	}
	return nil
}

// reconcileName is the seed reconcile of an existing weapon's or item's name
// (ADR 0023 step 10). The seed gives each of its languages to a name once:
// while every language the name has still reads as the seed's, it gains the
// seed languages it lacks that were never given, and those it has or gains
// are recorded as given. A name the player changed in any language is kept as
// is (and recorded nothing, so it is offered again once it reads as the
// seed's), a language the player removed after it was given is not put back,
// a row the seed does not list is untouched, an id the table no longer has is
// not added back (the player removed it), and no other field changes. A
// second run writes nothing.
func reconcileName(ctx context.Context, q Querier, n seededName, id string, seed localized.Text) error {
	var key int64
	err := q.QueryRowContext(ctx, n.keyQuery, id).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read name key: %w", err)
	}
	offered, err := langSet(ctx, q, `SELECT lang, '' FROM seeded_name_languages WHERE kind = ? AND id = ?`, n.kind, id)
	if err != nil {
		return fmt.Errorf("read seed offers: %w", err)
	}
	var fresh []string
	for _, lang := range seed.Languages() {
		if _, ok := offered[lang]; !ok {
			fresh = append(fresh, lang)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	stored, err := langSet(ctx, q, `SELECT lang, body FROM localized_texts WHERE text_id = ?`, key)
	if err != nil {
		return fmt.Errorf("read name %d: %w", key, err)
	}
	for lang, body := range stored {
		if seed[lang] != body {
			return nil
		}
	}
	if err := recordOffers(ctx, q, n, id, fresh); err != nil {
		return err
	}
	for _, lang := range fresh {
		if _, ok := stored[lang]; ok {
			continue
		}
		if err := insertText(ctx, q, key, lang, seed[lang]); err != nil {
			return err
		}
	}
	return nil
}

// langSet reads (lang, body) rows into a text.
func langSet(ctx context.Context, q Querier, query string, args ...any) (localized.Text, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := localized.Text{}
	for rows.Next() {
		var lang, body string
		if err := rows.Scan(&lang, &body); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[lang] = body
	}
	return out, errors.Join(rows.Err(), rows.Close())
}

// recordOffers records that the seed has given a name these languages.
func recordOffers(ctx context.Context, q Querier, n seededName, id string, langs []string) error {
	for _, lang := range langs {
		if _, err := q.ExecContext(ctx,
			`INSERT OR IGNORE INTO seeded_name_languages (kind, id, lang) VALUES (?, ?, ?)`, n.kind, id, lang,
		); err != nil {
			return fmt.Errorf("record seed offer %s %q %s: %w", n.kind, id, lang, err)
		}
	}
	return nil
}

// seedSkills reconciles the proficiencies and bard_skills tables with
// skills.json by id, in one transaction: a new id is inserted, an existing one
// takes the seed's ability, modifier, percentage and position (a corrected
// modifier, a percentage raised at level-up). Rows the seed no longer lists are
// kept, nothing is deleted, and a second run changes nothing. A missing file is
// an error only while both tables are empty.
func (d *DB) seedSkills(ctx context.Context, seedPath string) error {
	seed, err := seedfile.ReadSkills(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		var count int
		if cerr := d.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM proficiencies) + (SELECT COUNT(*) FROM bard_skills)").Scan(&count); cerr != nil {
			return fmt.Errorf("count skills: %w", cerr)
		}
		if count > 0 {
			return nil
		}
		return fmt.Errorf("skill tables are empty and their seed file is unreadable: %w", err)
	}
	if err != nil {
		return err
	}
	return d.WrapTx(ctx, func(q Querier) error {
		for i, p := range seed.Proficiencies {
			if _, err := q.ExecContext(ctx, `
				INSERT INTO proficiencies (id, position, ability, modifier) VALUES (?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET position = excluded.position, ability = excluded.ability, modifier = excluded.modifier`,
				p.ID, i, p.Ability, p.Modifier,
			); err != nil {
				return fmt.Errorf("upsert proficiency %q: %w", p.ID, err)
			}
		}
		for i, b := range seed.BardSkills {
			if _, err := q.ExecContext(ctx, `
				INSERT INTO bard_skills (id, position, percent) VALUES (?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET position = excluded.position, percent = excluded.percent`,
				b.ID, i, b.Percent,
			); err != nil {
				return fmt.Errorf("upsert bard skill %q: %w", b.ID, err)
			}
		}
		return nil
	})
}

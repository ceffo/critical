package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// Seed loads character.json, weapons.json, items.json and skills.json from seedsDir into their
// tables, in one transaction: character, weapons and items only if their table is empty (once
// filled, only the names of weapons and items are reconciled, reconcileName), skills reconciled
// by id. A missing character file is ignored. A missing weapons, items or skills file while its
// table is empty is an error: the catalog would be empty. Spells are not seeds: they come from
// the catalogue artifact (ImportCatalogue).
func (d *DB) Seed(ctx context.Context, seedsDir string) error { return seed(ctx, d, d.q, seedsDir) }

// seed is Seed over its ports: tx runs the transaction, q runs on the transaction tx carries.
func seed(ctx context.Context, tx domain.TxRunner, q storeq.Querier, seedsDir string) error {
	return tx.InTx(ctx, func(ctx context.Context) error {
		if err := seedCharacter(ctx, q, seedfile.Path(seedsDir, seedfile.CharacterFile)); err != nil {
			return fmt.Errorf("seed character: %w", err)
		}
		if err := seedWeapons(ctx, q, seedfile.Path(seedsDir, seedfile.WeaponsFile)); err != nil {
			return fmt.Errorf("seed weapons: %w", err)
		}
		if err := seedItems(ctx, q, seedfile.Path(seedsDir, seedfile.ItemsFile)); err != nil {
			return fmt.Errorf("seed items: %w", err)
		}
		if err := seedSkills(ctx, q, seedfile.Path(seedsDir, seedfile.SkillsFile)); err != nil {
			return fmt.Errorf("seed skills: %w", err)
		}
		return nil
	})
}

// seedCharacter inserts the character, then the rows that refer to it (its spell slots, buffs
// and round state): the foreign keys need the character first.
func seedCharacter(ctx context.Context, q storeq.Querier, seedPath string) error {
	count, err := q.CountCharacters(ctx)
	if err != nil {
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
	charID, err := q.InsertCharacter(ctx, storeq.InsertCharacterParams{
		Name: c.Name, Str: int64(c.Str), Dex: int64(c.Dex), Con: int64(c.Con), Int: int64(c.Int),
		Wis: int64(c.Wis), Cha: int64(c.Cha), HpMax: int64(c.HPMax), Thac0Base: int64(c.Thac0Base),
		Level: int64(c.Level), XpTotal: int64(c.XPTotal), SaveParaPoison: int64(c.SaveParaPoison),
		SaveRodStaffWand: int64(c.SaveRodStaffWand), SavePetriPoly: int64(c.SavePetriPoly),
		SaveBreath: int64(c.SaveBreath), SaveSpell: int64(c.SaveSpell), Languages: string(languages),
	})
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	slotsJSON, err := c.SlotsJSON()
	if err != nil {
		return err
	}
	if err := q.InsertSpellSlots(ctx, storeq.InsertSpellSlotsParams{CharacterID: charID, Slots: slotsJSON}); err != nil {
		return fmt.Errorf("insert spell_slots: %w", err)
	}
	if err := q.InsertActiveBuffs(ctx, charID); err != nil {
		return fmt.Errorf("insert active_buffs: %w", err)
	}
	if err := q.InsertRoundState(ctx, charID); err != nil {
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

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func seedWeapons(ctx context.Context, q storeq.Querier, seedPath string) error {
	count, err := q.CountWeapons(ctx)
	if err != nil {
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
		if err := q.InsertWeapon(ctx, storeq.InsertWeaponParams{
			ID: w.ID, Position: int64(i), Name: key, DamageSm: w.DamageSM, DamageL: w.DamageL,
			MagicToHit: int64(w.MagicToHit), MagicDamage: int64(w.MagicDamage), Kind: w.Kind,
			TwoHanded: boolInt(w.TwoHanded), AmmoKind: w.AmmoKind,
		}); err != nil {
			return fmt.Errorf("insert weapon %q: %w", w.ID, err)
		}
	}
	return nil
}

func seedItems(ctx context.Context, q storeq.Querier, seedPath string) error {
	count, err := q.CountItems(ctx)
	if err != nil {
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
		if err := q.InsertItem(ctx, storeq.InsertItemParams{
			ID: it.ID, Position: int64(i), Name: key, Type: it.Type, Behaviour: it.Behaviour,
			MagicBonus: int64(it.MagicBonus), AmmoKind: it.AmmoKind, Count: int64(it.Count),
			MaxCount: int64(it.Max()), EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes,
			Powers: it.PowersJSON,
		}); err != nil {
			return fmt.Errorf("insert item %q: %w", it.ID, err)
		}
	}
	return nil
}

// seededName says where a weapon's or an item's name key is read and the kind its seed offers
// are recorded under (seeded_name_languages).
type seededName struct {
	kind    string
	nameKey func(ctx context.Context, q storeq.Querier, id string) (int64, error)
}

var (
	weaponName = seededName{kind: "weapon", nameKey: func(ctx context.Context, q storeq.Querier, id string) (int64, error) {
		return q.WeaponNameKey(ctx, id)
	}}
	itemName = seededName{kind: "item", nameKey: func(ctx context.Context, q storeq.Querier, id string) (int64, error) {
		return q.ItemNameKey(ctx, id)
	}}
)

// addSeedText stores t under a new key, one more than the highest stored (as every writer of
// localized_texts), one row per language.
func addSeedText(ctx context.Context, q storeq.Querier, t localized.Text) (int64, error) {
	key, err := q.NextTextKey(ctx)
	if err != nil {
		return 0, fmt.Errorf("next text key: %w", err)
	}
	for _, lang := range t.Languages() {
		if err := insertText(ctx, q, key, lang, t[lang]); err != nil {
			return 0, err
		}
	}
	return key, nil
}

func insertText(ctx context.Context, q storeq.Querier, key int64, lang, body string) error {
	if err := q.InsertText(ctx, storeq.InsertTextParams{TextID: key, Lang: lang, Body: body}); err != nil {
		return fmt.Errorf("insert text %d %s: %w", key, lang, err)
	}
	return nil
}

// reconcileName is the seed reconcile of an existing weapon's or item's name (ADR 0023 step 10).
// The seed gives each of its languages to a name once: while every language the name has still
// reads as the seed's, it gains the seed languages it lacks that were never given, and those it
// has or gains are recorded as given. A name the player changed in any language is kept as is
// (and recorded nothing, so it is offered again once it reads as the seed's), a language the
// player removed after it was given is not put back, a row the seed does not list is untouched,
// an id the table no longer has is not added back (the player removed it), and no other field
// changes. A second run writes nothing.
func reconcileName(ctx context.Context, q storeq.Querier, n seededName, id string, seed localized.Text) error {
	key, err := n.nameKey(ctx, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read name key: %w", err)
	}
	offeredLangs, err := q.SeedOffers(ctx, storeq.SeedOffersParams{Kind: n.kind, ID: id})
	if err != nil {
		return fmt.Errorf("read seed offers: %w", err)
	}
	offered := map[string]bool{}
	for _, lang := range offeredLangs {
		offered[lang] = true
	}
	var fresh []string
	for _, lang := range seed.Languages() {
		if !offered[lang] {
			fresh = append(fresh, lang)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	rows, err := q.TextLanguages(ctx, key)
	if err != nil {
		return fmt.Errorf("read name %d: %w", key, err)
	}
	stored := localized.Text{}
	for _, r := range rows {
		stored[r.Lang] = r.Body
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

// recordOffers records that the seed has given a name these languages.
func recordOffers(ctx context.Context, q storeq.Querier, n seededName, id string, langs []string) error {
	for _, lang := range langs {
		if err := q.RecordSeedOffer(ctx, storeq.RecordSeedOfferParams{Kind: n.kind, ID: id, Lang: lang}); err != nil {
			return fmt.Errorf("record seed offer %s %q %s: %w", n.kind, id, lang, err)
		}
	}
	return nil
}

// seedSkills reconciles the proficiencies and bard_skills tables with skills.json by id: a new id
// is inserted, an existing one takes the seed's ability, modifier, percentage and position (a
// corrected modifier, a percentage raised at level-up). Rows the seed no longer lists are kept,
// nothing is deleted, and a second run changes nothing. A missing file is an error only while
// both tables are empty.
func seedSkills(ctx context.Context, q storeq.Querier, seedPath string) error {
	seed, err := seedfile.ReadSkills(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		count, cerr := q.CountSkills(ctx)
		if cerr != nil {
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
	for i, p := range seed.Proficiencies {
		if err := q.UpsertProficiency(ctx, storeq.UpsertProficiencyParams{
			ID: p.ID, Position: int64(i), Ability: p.Ability, Modifier: int64(p.Modifier),
		}); err != nil {
			return fmt.Errorf("upsert proficiency %q: %w", p.ID, err)
		}
	}
	for i, b := range seed.BardSkills {
		if err := q.UpsertBardSkill(ctx, storeq.UpsertBardSkillParams{
			ID: b.ID, Position: int64(i), Percent: int64(b.Percent),
		}); err != nil {
			return fmt.Errorf("upsert bard skill %q: %w", b.ID, err)
		}
	}
	return nil
}

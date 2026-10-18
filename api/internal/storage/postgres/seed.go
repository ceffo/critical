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

// Seed loads the seed files of seedsDir, in one transaction. character.json is inserted only
// into an empty characters table. The other seeds are the starting set of the seed character,
// the character character.json names (seedOwner): its weapons and items only while it has none
// (once it has some, only the names of weapons and items are reconciled, reconcileName), its
// skills reconciled by id, its spellbook (spellbook: the spellbook seed's catalogue keys, in
// order, checked against the catalogue by the caller before anything is written) only while it
// has none. A character created later starts with nothing. Every row has an owner by then: the
// migration run gave the rows without one to the seed character (ownRows). A missing character
// file is ignored. A missing weapons,
// items or skills file while the seed character has none is an error: its sheet would be empty.
// Spells are not seeds: they come from the catalogue artifact (ImportCatalogue).
func (d *DB) Seed(ctx context.Context, seedsDir string, spellbook []string) error {
	return seed(ctx, d, d.q, seedsDir, spellbook)
}

// seed is Seed over its ports: tx runs the transaction, q runs on the transaction tx carries.
func seed(ctx context.Context, tx domain.TxRunner, q storeq.Querier, seedsDir string, spellbook []string) error {
	return tx.InTx(ctx, func(ctx context.Context) error {
		characterPath := seedfile.Path(seedsDir, seedfile.CharacterFile)
		if err := seedCharacter(ctx, q, characterPath); err != nil {
			return fmt.Errorf("seed character: %w", err)
		}
		owner, ok, err := seedOwner(ctx, q, characterPath)
		if err != nil || !ok {
			return err
		}
		if err := seedWeapons(ctx, q, owner, seedfile.Path(seedsDir, seedfile.WeaponsFile)); err != nil {
			return fmt.Errorf("seed weapons: %w", err)
		}
		if err := seedItems(ctx, q, owner, seedfile.Path(seedsDir, seedfile.ItemsFile)); err != nil {
			return fmt.Errorf("seed items: %w", err)
		}
		if err := seedSkills(ctx, q, owner, seedfile.Path(seedsDir, seedfile.SkillsFile)); err != nil {
			return fmt.Errorf("seed skills: %w", err)
		}
		if err := seedSpellbook(ctx, q, owner, spellbook); err != nil {
			return fmt.Errorf("seed spellbook: %w", err)
		}
		return nil
	})
}

// seedOwner is the seed character: the character named in character.json (the lowest id of
// that name). ok is false when there is none (no character file, or no character of its name);
// the equipment seeds are then not applied.
func seedOwner(ctx context.Context, q storeq.Querier, characterPath string) (int64, bool, error) {
	c, found, err := seedfile.ReadCharacter(characterPath)
	if err != nil || !found {
		return 0, false, err
	}
	id, err := q.SeedCharacterID(ctx, c.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find seed character: %w", err)
	}
	return id, true, nil
}

// ownRows gives every weapon, item, proficiency and bard skill without an owner to the seed
// character, with every count, position and field as it was: the data step before
// ownerKeysMigration makes the owner required. Rows without an owner and no seed character (the
// name in character.json changed) are an error rather than belong to no one: the migration run
// rolls back and the server does not start.
func ownRows(ctx context.Context, q storeq.Querier, characterPath string) error {
	owner, ok, err := seedOwner(ctx, q, characterPath)
	if err != nil {
		return err
	}
	if !ok {
		unowned, err := q.CountUnowned(ctx)
		if err != nil {
			return fmt.Errorf("count unowned rows: %w", err)
		}
		if unowned > 0 {
			return fmt.Errorf("%d weapons, items or skills have no owner and no character is named as in %s: "+
				"they would belong to no one", unowned, characterPath)
		}
		return nil
	}
	for _, adopt := range []struct {
		table string
		run   func(context.Context, int64) error
	}{
		{"weapons", q.AdoptWeapons},
		{"items", q.AdoptItems},
		{"proficiencies", q.AdoptProficiencies},
		{"bard_skills", q.AdoptBardSkills},
	} {
		if err := adopt.run(ctx, owner); err != nil {
			return fmt.Errorf("give unowned %s to character %d: %w", adopt.table, owner, err)
		}
	}
	return nil
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

func seedWeapons(ctx context.Context, q storeq.Querier, owner int64, seedPath string) error {
	count, err := q.CountWeapons(ctx, owner)
	if err != nil {
		return fmt.Errorf("count weapons: %w", err)
	}
	weapons, err := seedfile.ReadWeapons(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		if count > 0 {
			return nil
		}
		return fmt.Errorf("the seed character has no weapons and their seed file is unreadable: %w", err)
	}
	if err != nil {
		return err
	}
	if count > 0 {
		for _, w := range weapons {
			if err := reconcileName(ctx, q, weaponName, owner, w.ID, w.Name.Trimmed()); err != nil {
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
			CharacterID: owner, ID: w.ID, Position: int64(i), Name: key, DamageSm: w.DamageSM, DamageL: w.DamageL,
			MagicToHit: int64(w.MagicToHit), MagicDamage: int64(w.MagicDamage), Kind: w.Kind,
			TwoHanded: boolInt(w.TwoHanded), AmmoKind: w.AmmoKind,
		}); err != nil {
			return fmt.Errorf("insert weapon %q: %w", w.ID, err)
		}
	}
	return nil
}

func seedItems(ctx context.Context, q storeq.Querier, owner int64, seedPath string) error {
	count, err := q.CountItems(ctx, owner)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}
	items, err := seedfile.ReadItems(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		if count > 0 {
			return nil
		}
		return fmt.Errorf("the seed character has no items and their seed file is unreadable: %w", err)
	}
	if err != nil {
		return err
	}
	if count > 0 {
		for _, it := range items {
			if err := reconcileName(ctx, q, itemName, owner, it.ID, it.Name.Trimmed()); err != nil {
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
			CharacterID: owner, ID: it.ID, Position: int64(i), Name: key, Type: it.Type, Behaviour: it.Behaviour,
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
// are recorded under (seeded_name_languages). Only the seed character's weapons and items are
// seeded, so an offer is recorded by kind and id.
type seededName struct {
	kind    string
	nameKey func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error)
}

var (
	weaponName = seededName{kind: "weapon", nameKey: func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error) {
		return q.WeaponNameKey(ctx, storeq.WeaponNameKeyParams{CharacterID: owner, ID: id})
	}}
	itemName = seededName{kind: "item", nameKey: func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error) {
		return q.ItemNameKey(ctx, storeq.ItemNameKeyParams{CharacterID: owner, ID: id})
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
func reconcileName(ctx context.Context, q storeq.Querier, n seededName, owner int64, id string, seed localized.Text) error {
	key, err := n.nameKey(ctx, q, owner, id)
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

// seedSkills reconciles the seed character's proficiencies and bard skills with skills.json by
// id (ADR 0014): a new id is inserted, an existing one takes the seed's ability, modifier,
// percentage and position (a corrected modifier, a percentage raised at level-up). Rows the seed
// no longer lists are kept, nothing is deleted, and a second run changes nothing. A missing file
// is an error only while the character has no skill.
func seedSkills(ctx context.Context, q storeq.Querier, owner int64, seedPath string) error {
	seed, err := seedfile.ReadSkills(seedPath)
	if errors.Is(err, seedfile.ErrUnreadable) {
		count, cerr := q.CountSkills(ctx, owner)
		if cerr != nil {
			return fmt.Errorf("count skills: %w", cerr)
		}
		if count > 0 {
			return nil
		}
		return fmt.Errorf("the seed character has no skills and their seed file is unreadable: %w", err)
	}
	if err != nil {
		return err
	}
	for i, p := range seed.Proficiencies {
		if err := q.UpsertProficiency(ctx, storeq.UpsertProficiencyParams{
			CharacterID: owner, ID: p.ID, Position: int64(i), Ability: p.Ability, Modifier: int64(p.Modifier),
		}); err != nil {
			return fmt.Errorf("upsert proficiency %q: %w", p.ID, err)
		}
	}
	for i, b := range seed.BardSkills {
		if err := q.UpsertBardSkill(ctx, storeq.UpsertBardSkillParams{
			CharacterID: owner, ID: b.ID, Position: int64(i), Percent: int64(b.Percent),
		}); err != nil {
			return fmt.Errorf("upsert bard skill %q: %w", b.ID, err)
		}
	}
	return nil
}

// seedSpellbook writes the spellbook seed as the seed character's spellbook, once: only while
// that character has none, as its weapons and items (spells are then learned and removed in the
// app, ADR 0030 step 20, never by the seed). keys were checked against the catalogue by the
// caller (spellbookadpt.ReadSeed), so only catalogue spells are written.
func seedSpellbook(ctx context.Context, q storeq.Querier, owner int64, keys []string) error {
	count, err := q.CountSpellbookEntries(ctx, owner)
	if err != nil {
		return fmt.Errorf("count spellbook entries: %w", err)
	}
	if count > 0 {
		return nil
	}
	for i, k := range keys {
		if err := q.InsertSpellbookEntry(ctx, storeq.InsertSpellbookEntryParams{
			CharacterID: owner, SpellKey: k, Position: int64(i),
		}); err != nil {
			return fmt.Errorf("insert spellbook entry %q: %w", k, err)
		}
	}
	return nil
}

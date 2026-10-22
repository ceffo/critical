package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// Seed loads the seed files of seedsDir, in one transaction. character.json is inserted only
// into an empty characters table. The other seeds are the starting set of the seed character,
// the character character.json names (seedOwner): its equipment (the weapons and items seeds as one
// possessions list, ADR 0033) only while it has none (once it has some, only the names of the
// seeded pieces are reconciled, reconcileName), its
// skills reconciled by id, its spellbook (spellbook: the spellbook seed's catalogue keys, in
// order, checked against the catalogue by the caller before anything is written) only while it
// has none. A character created later starts with nothing. Every row has an owner by then: the
// migration run gave the rows without one to the seed character (ownRows). A missing character
// file is ignored. A missing weapons,
// items or skills file while the seed character has none is an error: its sheet would be empty.
// equipment is the catalogue's: its weapons, which the seeded weapons are matched to (ADR 0033 step
// 26.4), and its body armours, from which a seed character with skill points gets the armour it
// entered, worn, when its equipment is seeded (step 14's carry-over, as an upgraded database has).
// Spells are not seeds: they come from the catalogue artifact (ImportCatalogue).
func (d *DB) Seed(ctx context.Context, seedsDir string, spellbook []string, equipment ruletablesvc.Equipment, defs *classes.Registry) error {
	return seed(ctx, d, d.q, seedsDir, spellbook, equipment, defs)
}

// seed is Seed over its ports: tx runs the transaction, q runs on the transaction tx carries.
func seed(ctx context.Context, tx domain.TxRunner, q storeq.Querier, seedsDir string, spellbook []string, equipment ruletablesvc.Equipment, defs *classes.Registry) error {
	return tx.InTx(ctx, func(ctx context.Context) error {
		characterPath := seedfile.Path(seedsDir, seedfile.CharacterFile)
		if err := seedCharacter(ctx, q, characterPath); err != nil {
			return fmt.Errorf("seed character: %w", err)
		}
		owner, ok, err := seedOwner(ctx, q, characterPath)
		if err != nil || !ok {
			return err
		}
		if err := seedEquipment(ctx, q, owner, seedfile.Path(seedsDir, seedfile.ItemsFile),
			seedfile.Path(seedsDir, seedfile.WeaponsFile), equipment, defs); err != nil {
			return fmt.Errorf("seed equipment: %w", err)
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

// typeSeedWeapons gives the weapons stored before weapon types the type the seeds name for their
// id, for every character holding one: the data step after weaponTypeMigration. A weapon that
// already has a type keeps it, so a rerun changes nothing and no type the owner set is lost.
func typeSeedWeapons(ctx context.Context, q storeq.Querier, weaponsPath string) error {
	weapons, err := seedfile.ReadWeapons(weaponsPath)
	if err != nil {
		if errors.Is(err, seedfile.ErrUnreadable) {
			return nil // no seeds, no seeded weapon to type
		}
		return err
	}
	for _, w := range weapons {
		if w.WeaponType == "" {
			continue
		}
		if err := q.TypeWeaponIfUntyped(ctx, storeq.TypeWeaponIfUntypedParams{ID: w.ID, WeaponType: w.WeaponType}); err != nil {
			return fmt.Errorf("type weapon %q: %w", w.ID, err)
		}
	}
	return nil
}

// raceSeedCharacter gives the seed character stored before races had codes the race its seed
// names: the data step after raceMigration. A character that already has a race keeps it, so a
// rerun changes nothing and no race set since is lost; no seed character, or a seed without a
// race, changes nothing.
func raceSeedCharacter(ctx context.Context, q storeq.Querier, characterPath string) error {
	c, found, err := seedfile.ReadCharacter(characterPath)
	if err != nil || !found || c.Race == "" {
		return err
	}
	owner, ok, err := seedOwner(ctx, q, characterPath)
	if err != nil || !ok {
		return err
	}
	if err := q.SetRaceIfUnset(ctx, storeq.SetRaceIfUnsetParams{ID: owner, RaceCode: c.Race}); err != nil {
		return fmt.Errorf("set the race of character %d: %w", owner, err)
	}
	return nil
}

// pointSeedCharacter gives the seed character stored before bards entered skill points the points
// its seed names (the ones that give his sheet's percentages, ADR 0030 steps 18 and 24): the data
// step after bardSkillsMigration. A character that already has points keeps them, so a rerun
// changes nothing and no spend since is lost; no seed character, or a seed without points, changes
// nothing.
func pointSeedCharacter(ctx context.Context, q storeq.Querier, characterPath string) error {
	c, found, err := seedfile.ReadCharacter(characterPath)
	if err != nil || !found || len(c.SkillPoints) == 0 {
		return err
	}
	owner, ok, err := seedOwner(ctx, q, characterPath)
	if err != nil || !ok {
		return err
	}
	// The step runs before the skill sets column exists: it writes the legacy flat column, the one
	// the carry-over of skillSetsMigration reads (a seed has one skill set).
	flat := map[string]int{}
	for _, points := range c.SkillPoints {
		maps.Copy(flat, points)
	}
	points, err := skillPointsJSON(flat)
	if err != nil {
		return err
	}
	if err := q.SetSkillPointsIfUnset(ctx, storeq.SetSkillPointsIfUnsetParams{ID: owner, SkillPoints: points}); err != nil {
		return fmt.Errorf("set the skill points of character %d: %w", owner, err)
	}
	return nil
}

// skillPointsJSON is points as the legacy characters.skill_points stores them ("{}" for none).
func skillPointsJSON(points map[string]int) (string, error) {
	if len(points) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(points)
	if err != nil {
		return "", fmt.Errorf("marshal skill points: %w", err)
	}
	return string(b), nil
}

// skillSetsJSON is points as characters.skill_sets stores them ("{}" for none).
func skillSetsJSON(points skills.SetPoints) (string, error) {
	if len(points) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(points)
	if err != nil {
		return "", fmt.Errorf("marshal skill sets: %w", err)
	}
	return string(b), nil
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
	points, err := skillSetsJSON(c.SkillPoints)
	if err != nil {
		return err
	}
	// ADR 0037 step 16: a new seed character gets the seed's picks; once stored they are the
	// player's, never reconciled.
	picks, err := characterdal.EncodePicks(c.Picks())
	if err != nil {
		return err
	}
	psionicPicks, err := characterdal.EncodePsionicPicks(c.PsionicPicks)
	if err != nil {
		return err
	}
	charID, err := q.InsertCharacter(ctx, storeq.InsertCharacterParams{
		Name: c.Name, Str: int64(c.Str), Dex: int64(c.Dex), Con: int64(c.Con), Int: int64(c.Int),
		Wis: int64(c.Wis), Cha: int64(c.Cha), HpMax: int64(c.HPMax), Thac0Base: int64(c.Thac0Base),
		Level: int64(c.Level), XpTotal: int64(c.XPTotal), SaveParaPoison: int64(c.SaveParaPoison),
		SaveRodStaffWand: int64(c.SaveRodStaffWand), SavePetriPoly: int64(c.SavePetriPoly),
		SaveBreath: int64(c.SaveBreath), SaveSpell: int64(c.SaveSpell), Languages: string(languages),
		RaceCode: c.Race, SkillSets: points, Alignment: c.Alignment, Sex: c.Sex, ProficiencyPicks: picks, Class: c.Class, PsionicPicks: psionicPicks,
	})
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	// ADR 0042 step 1: the seed's class, realized at its sheet level (step 7.5), its XP as the
	// entry's starting XP.
	if err := q.InsertClassEntry(ctx, storeq.InsertClassEntryParams{
		CharacterID: charID, Position: 0, Class: c.Class, RealizedLevel: int64(c.Level),
		StartingXp: int64(c.XPTotal), State: string(classes.EntryActive),
	}); err != nil {
		return fmt.Errorf("insert class entry: %w", err)
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

// seedEquipment seeds the seed character's equipment (ADR 0033 step 1): while it holds no piece,
// the items and weapons seeds converted as an upgraded database's are (seedfile.Equipment), with
// their review rows; once it holds some, only the names of the seeded pieces are reconciled
// (reconcileName). A seed file that is unreadable is an error only while the character holds none.
func seedEquipment(ctx context.Context, q storeq.Querier, owner int64, itemsPath, weaponsPath string, equipment ruletablesvc.Equipment, defs *classes.Registry) error {
	count, err := q.CountItems(ctx, owner)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}
	items, err := seedfile.ReadItems(itemsPath)
	if errors.Is(err, seedfile.ErrUnreadable) && count == 0 {
		return fmt.Errorf("the seed character has no equipment and the items seed file is unreadable: %w", err)
	}
	if err != nil && !errors.Is(err, seedfile.ErrUnreadable) {
		return err
	}
	weapons, err := seedfile.ReadWeapons(weaponsPath)
	if errors.Is(err, seedfile.ErrUnreadable) && count == 0 {
		return fmt.Errorf("the seed character has no equipment and the weapons seed file is unreadable: %w", err)
	}
	if err != nil && !errors.Is(err, seedfile.ErrUnreadable) {
		return err
	}
	if count > 0 {
		for _, it := range items {
			if err := reconcileName(ctx, q, itemName, owner, it.ID, it.Name.Trimmed()); err != nil {
				return fmt.Errorf("item %q: %w", it.ID, err)
			}
		}
		for _, w := range weapons {
			if err := reconcileName(ctx, q, weaponName, owner, w.ID, w.Name.Trimmed()); err != nil {
				return fmt.Errorf("weapon %q: %w", w.ID, err)
			}
		}
		return nil
	}
	conv, err := seedfile.Equipment(items, weapons, equipment.Weapons)
	if err != nil {
		return err
	}
	for i, p := range conv.Pieces {
		// The items' pieces come first, then the weapons'.
		n := itemName
		if i >= len(items) {
			n = weaponName
		}
		key, err := addSeedText(ctx, q, p.Name)
		if err != nil {
			return fmt.Errorf("%s %q: %w", n.kind, p.ID, err)
		}
		if err := recordOffers(ctx, q, n, p.ID, p.Name.Languages()); err != nil {
			return fmt.Errorf("%s %q: %w", n.kind, p.ID, err)
		}
		if err := insertPiece(ctx, q, owner, p, key); err != nil {
			return err
		}
	}
	if err := insertReviews(ctx, q, owner, conv.Reviews); err != nil {
		return err
	}
	// ADR 0033 step 14: the seed character with skill points wears the armour it entered, as the
	// carry-over gives an upgraded database's. Without the catalogue's armours (before format 7)
	// it wears none, as its weapons stay unmatched.
	if len(equipment.Armour) == 0 {
		return nil
	}
	return carryCharacter(ctx, q, owner, equipment.Armour, defs)
}

// seededName is the kind a seeded piece's offers are recorded under (seeded_name_languages): the
// file it was seeded from, item or weapon; its name key is read among the pieces. Only the seed
// character's equipment is seeded, so an offer is recorded by kind and id.
type seededName struct {
	kind    string
	nameKey func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error)
}

var (
	weaponName = seededName{kind: "weapon", nameKey: func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error) {
		return q.ItemNameKey(ctx, storeq.ItemNameKeyParams{CharacterID: owner, ID: id})
	}}
	// An item the conversion renamed (it shared its id with a weapon) is read under its new id; its
	// offers stay recorded under the seed's id.
	itemName = seededName{kind: "item", nameKey: func(ctx context.Context, q storeq.Querier, owner int64, id string) (int64, error) {
		renamed, err := q.RenamedItemID(ctx, storeq.RenamedItemIDParams{CharacterID: owner, OldID: id})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("read the item's rename: %w", err)
		}
		if err == nil {
			id = renamed
		}
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

// seedSkills reconciles the seed character's bard skills with skills.json by id (ADR 0014): a new
// id is inserted, an existing one takes the seed's percentage and position. Rows the seed no
// longer lists are kept, nothing is deleted, and a second run changes nothing. A missing file is
// an error only while the character has no skill. The proficiencies are no longer reconciled
// (ADR 0037 step 16): they are picks, the seed character's from character.json.
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
// caller (spellbookadpt.ReadSeed), so only catalogue spells are written. Each entry's origin is
// starting (ADR 0030 step 20.5: Kelsen's imported book).
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
			CharacterID: owner, SpellKey: k, Position: int64(i), Origin: string(spellbook.OriginStarting),
		}); err != nil {
			return fmt.Errorf("insert spellbook entry %q: %w", k, err)
		}
	}
	return nil
}

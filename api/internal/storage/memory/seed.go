package memory

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// Seed loads the seed files of seedsDir (seedfile, checked by the same rules
// as the database's seed) into the store, in one transaction. The store
// starts empty, so this is the database seed's first run: the character (a
// missing character file is ignored), then its starting set, as the seed
// character's: the equipment (a missing file is an error: its sheet
// would be empty), the skills and the spellbook (keys: the spellbook seed's
// catalogue keys in order, checked against the catalogue by the caller; each
// entry's origin is starting, ADR 0030 step 20.5). Without a character file nothing owns the starting set, so it is
// not loaded.
func (s *Store) Seed(ctx context.Context, seedsDir string, keys []string, equipment ruletablesvc.Equipment) error {
	return s.InTx(ctx, func(context.Context) error {
		t := s.t
		owner, ok, err := t.seedCharacter(seedfile.Path(seedsDir, seedfile.CharacterFile))
		if err != nil {
			return fmt.Errorf("seed character: %w", err)
		}
		if !ok {
			return nil
		}
		if err := t.seedEquipment(owner, seedfile.Path(seedsDir, seedfile.ItemsFile),
			seedfile.Path(seedsDir, seedfile.WeaponsFile), equipment.Weapons); err != nil {
			return fmt.Errorf("seed equipment: %w", err)
		}
		if err := t.carrySkillArmour(owner, equipment.Armour); err != nil {
			return fmt.Errorf("seed skill armour: %w", err)
		}
		if err := t.seedSkills(owner, seedfile.Path(seedsDir, seedfile.SkillsFile)); err != nil {
			return fmt.Errorf("seed skills: %w", err)
		}
		if len(t.spellbooks[owner]) == 0 { // once, while it has none (as the database's seed)
			book := make([]spellbook.Entry, len(keys))
			for i, k := range keys {
				book[i] = spellbook.Entry{Key: k, Origin: spellbook.OriginStarting}
			}
			t.spellbooks[owner] = book
		}
		return nil
	})
}

// seedCharacter inserts the character of the character file into the empty
// store and returns its id; ok is false without a character file, or when
// the store already holds characters (seeded already: the starting set is
// applied once).
func (t *tables) seedCharacter(path string) (id int64, ok bool, err error) {
	if len(t.characters) > 0 {
		return 0, false, nil
	}
	c, ok, err := seedfile.ReadCharacter(path)
	if err != nil || !ok {
		return 0, false, err
	}
	// The slots are read as the spell slot adapter reads its stored list.
	encoded, err := c.SlotsJSON()
	if err != nil {
		return 0, false, err
	}
	var raw []struct {
		Level             int    `json:"level"`
		PreparedSpellID   string `json:"prepared_spell_id"`
		PreparedSpellName string `json:"prepared_spell_name"`
		Used              bool   `json:"used"`
		Extra             bool   `json:"extra"`
	}
	if err := json.Unmarshal([]byte(encoded), &raw); err != nil {
		return 0, false, fmt.Errorf("parse slots: %w", err)
	}
	slots := make([]spellsvc.SpellSlot, len(raw))
	for i, r := range raw {
		slots[i] = spellsvc.SpellSlot{
			Level: r.Level, SpellID: r.PreparedSpellID, PreparedSpellName: r.PreparedSpellName, Used: r.Used, Extra: r.Extra,
		}
	}
	id = nextID(&t.characterSeq)
	t.characters[id] = charactersvc.Character{
		ID: id, Name: c.Name, Str: c.Str, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha,
		HPMax: c.HPMax, XPTotal: c.XPTotal, Class: c.Class, Race: c.Race, SkillPoints: c.SkillPoints,
		Proficiencies: c.Picks(),
		Sheet: charactersvc.Sheet{Level: c.Level, THAC0: c.Thac0Base, Saves: charactersvc.Saves{
			ParaPoison: c.SaveParaPoison, RodStaffWand: c.SaveRodStaffWand, PetriPoly: c.SavePetriPoly,
			Breath: c.SaveBreath, Spell: c.SaveSpell,
		}},
		Details: charactersvc.Details{Alignment: c.Alignment, Sex: c.Sex, Languages: append([]string{}, c.Languages...)},
	}
	t.slots[id] = slots
	t.buffs[id] = []buffsvc.BuffEffect{}
	t.rounds[id] = roundState{}
	return id, true, nil
}

// seedEquipment stores the items and weapons seeds as the database's seed
// does: converted into pieces (seedfile.Equipment), with their review rows.
func (t *tables) seedEquipment(owner int64, itemsPath, weaponsPath string, catalogue []ruletablesvc.Weapon) error {
	items, err := seedfile.ReadItems(itemsPath)
	if err != nil {
		return err
	}
	weapons, err := seedfile.ReadWeapons(weaponsPath)
	if err != nil {
		return err
	}
	conv, err := seedfile.Equipment(items, weapons, catalogue)
	if err != nil {
		return err
	}
	for _, p := range conv.Pieces {
		st, err := toStored(p, nextID(&t.rowSeq))
		if err != nil {
			return err
		}
		t.items[owned{owner, p.ID}] = st
	}
	for _, r := range conv.Reviews {
		t.reviews[review{owner, r.ItemID, r.Reason}] = true
	}
	return nil
}

// seedSkills checks the skills seed as the database's seed does. The seed's
// bard skill percentages are the sheet's figures (ADR 0030 step 24), not
// inputs: the bard's skills are derived from its points (step 18), so the
// memory store keeps none of them; the proficiencies are the character
// seed's picks (ADR 0037 step 16).
func (t *tables) seedSkills(_ int64, path string) error {
	_, err := seedfile.ReadSkills(path)
	return err
}

// ImportCatalogue imports the spell catalogue artifact (ADR 0021 step 5,
// after catalogueadpt read and checked it). The store starts empty, so this
// is a database's first import: every artifact spell is stored by its key
// (step 5.4.2) with its typed stats parsed from the fallback-language stat
// texts (step 5.6), and the format and version are recorded (step 5.5). The
// same version again writes nothing (step 5.3). seedRefs only guard the
// spells an earlier catalogue left (step 5.4.3): a store lives as long as its
// process and imports once, so another version on a store that holds one is
// refused instead.
func (s *Store) ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, _ []catalogue.Ref) error {
	return s.InTx(ctx, func(context.Context) error {
		t := s.t
		if v := t.catalogue; v != nil {
			if v.format == cat.Format && v.version == cat.Version {
				return nil
			}
			return fmt.Errorf("spell catalogue: the in-memory store holds catalogue version %s, refusing to import version %s",
				v.version, cat.Version)
		}
		spells := make(map[string]spellsvc.Spell, len(cat.Spells))
		for _, sp := range cat.Spells {
			if _, dup := spells[sp.ID]; dup {
				return errDuplicate("spells", sp.ID)
			}
			spells[sp.ID] = spellsvc.Spell{
				ID: sp.ID, Class: sp.Class, Level: sp.Level, Source: sp.Source,
				Name: stored(sp.Name), School: stored(sp.School), Sphere: stored(sp.Sphere),
				CastingTime: stored(sp.CastingTime), Range: stored(sp.Range), Duration: stored(sp.Duration),
				AreaOfEffect: stored(sp.AreaOfEffect), Components: stored(sp.Components),
				SavingThrow: stored(sp.SavingThrow), Description: stored(sp.Description),
				Stats: spellstats.Parse(spellstats.Raw{
					CastingTime: sp.CastingTime.Default(), Range: sp.Range.Default(), Duration: sp.Duration.Default(),
					AreaOfEffect: sp.AreaOfEffect.Default(), Components: sp.Components.Default(),
					SavingThrow: sp.SavingThrow.Default(),
				}),
			}
		}
		t.spells = spells
		t.profDescs = nil
		for _, d := range cat.ProficiencyDescriptions {
			t.profDescs = append(t.profDescs, catalogue.ProficiencyDescription{ID: d.ID, Description: stored(d.Description)})
		}
		t.catalogue = &catalogueVersion{format: cat.Format, version: cat.Version}
		return nil
	})
}

// ProficiencyDescriptions returns the imported proficiency descriptions as language maps (ADR
// 0021 step 16), in the artifact's order: what the database's read gives after its import.
func (s *Store) ProficiencyDescriptions(context.Context) ([]catalogue.ProficiencyDescription, error) {
	out := make([]catalogue.ProficiencyDescription, len(s.t.profDescs))
	copy(out, s.t.profDescs)
	return out, nil
}

// carrySkillArmour gives the seed character the armour it entered for its
// skills (none: leather), worn, as the database's seed does (ADR 0033 step
// 14, itemsvc.CarrySkillArmour). Without the catalogue's armours (before
// format 7) it wears none.
func (t *tables) carrySkillArmour(owner int64, armours []ruletablesvc.Armour) error {
	if len(armours) == 0 {
		return nil
	}
	var pieces []itemsvc.Item
	for k, st := range t.items {
		if k.characterID == owner {
			pieces = append(pieces, st.Item)
		}
	}
	slices.SortFunc(pieces, func(a, b itemsvc.Item) int { return cmp.Compare(a.Position, b.Position) })
	co, err := itemsvc.CarrySkillArmour(t.characters[owner].Class, t.skillArmors[owner], pieces, armours)
	if err != nil {
		return err
	}
	switch {
	case co.Wear != "":
		st := t.items[owned{owner, co.Wear}]
		st.State = itemsvc.StateWorn
		t.items[owned{owner, co.Wear}] = st
	case co.Add != nil:
		st, err := toStored(*co.Add, nextID(&t.rowSeq))
		if err != nil {
			return err
		}
		t.items[owned{owner, co.Add.ID}] = st
		if co.Review != nil {
			t.reviews[review{owner, co.Review.ItemID, co.Review.Reason}] = true
		}
	}
	return nil
}

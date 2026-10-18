package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// Seed loads the seed files of seedsDir (seedfile, checked by the same rules
// as the database's seed) into the store, in one transaction. The store
// starts empty, so this is the database seed's first run: the character (a
// missing character file is ignored), then its starting set, as the seed
// character's: the weapons and items (a missing file is an error: its sheet
// would be empty), the skills and the spellbook (spellbook: the spellbook
// seed's catalogue keys in order, checked against the catalogue by the
// caller). Without a character file nothing owns the starting set, so it is
// not loaded.
func (s *Store) Seed(ctx context.Context, seedsDir string, spellbook []string) error {
	return s.InTx(ctx, func(context.Context) error {
		t := s.t
		owner, ok, err := t.seedCharacter(seedfile.Path(seedsDir, seedfile.CharacterFile))
		if err != nil {
			return fmt.Errorf("seed character: %w", err)
		}
		if !ok {
			return nil
		}
		if err := t.seedWeapons(owner, seedfile.Path(seedsDir, seedfile.WeaponsFile)); err != nil {
			return fmt.Errorf("seed weapons: %w", err)
		}
		if err := t.seedItems(owner, seedfile.Path(seedsDir, seedfile.ItemsFile)); err != nil {
			return fmt.Errorf("seed items: %w", err)
		}
		if err := t.seedSkills(owner, seedfile.Path(seedsDir, seedfile.SkillsFile)); err != nil {
			return fmt.Errorf("seed skills: %w", err)
		}
		if len(t.spellbooks[owner]) == 0 { // once, while it has none (as the database's seed)
			t.spellbooks[owner] = append([]string{}, spellbook...)
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
		HPMax: c.HPMax, Thac0Base: c.Thac0Base, Level: c.Level, XPTotal: c.XPTotal,
		SaveParaPoison: c.SaveParaPoison, SaveRodStaffWand: c.SaveRodStaffWand, SavePetriPoly: c.SavePetriPoly,
		SaveBreath: c.SaveBreath, SaveSpell: c.SaveSpell, Class: seedfile.DefaultClass,
		Details: charactersvc.Details{Languages: append([]string{}, c.Languages...)},
	}
	t.slots[id] = slots
	t.buffs[id] = []buffsvc.BuffEffect{}
	t.rounds[id] = roundState{}
	return id, true, nil
}

func (t *tables) seedWeapons(owner int64, path string) error {
	weapons, err := seedfile.ReadWeapons(path)
	if err != nil {
		return err
	}
	for i, w := range weapons {
		t.weapons[owned{owner, w.ID}] = storedWeapon{row: nextID(&t.rowSeq), Weapon: weaponsvc.Weapon{
			ID: w.ID, Position: i, Name: stored(w.Name.Trimmed()), DamageSM: w.DamageSM, DamageL: w.DamageL,
			MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: w.Kind, TwoHanded: w.TwoHanded,
			AmmoKind: w.AmmoKind,
		}}
	}
	return nil
}

func (t *tables) seedItems(owner int64, path string) error {
	items, err := seedfile.ReadItems(path)
	if err != nil {
		return err
	}
	for i, it := range items {
		t.items[owned{owner, it.ID}] = storedItem{row: nextID(&t.rowSeq), powers: it.PowersJSON, Item: itemsvc.Item{
			ID: it.ID, Position: i, Name: stored(it.Name.Trimmed()), Type: it.Type,
			Behaviour: itemsvc.Behaviour(it.Behaviour), MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind,
			Count: it.Count, MaxCount: it.Max(), EffectDice: it.EffectDice, EffectSave: it.EffectSave,
			Notes: it.Notes,
		}}
	}
	return nil
}

func (t *tables) seedSkills(owner int64, path string) error {
	seed, err := seedfile.ReadSkills(path)
	if err != nil {
		return err
	}
	profs := make([]skillsvc.ProficiencyDef, len(seed.Proficiencies))
	for i, p := range seed.Proficiencies {
		profs[i] = skillsvc.ProficiencyDef{ID: p.ID, Ability: skillsvc.Ability(p.Ability), Modifier: p.Modifier}
	}
	skills := make([]skillsvc.BardSkillDef, len(seed.BardSkills))
	for i, b := range seed.BardSkills {
		skills[i] = skillsvc.BardSkillDef{ID: b.ID, Percent: b.Percent}
	}
	t.proficiencies[owner], t.bardSkills[owner] = profs, skills
	return nil
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
		t.catalogue = &catalogueVersion{format: cat.Format, version: cat.Version}
		return nil
	})
}

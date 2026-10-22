// Package seedfile reads and checks the seed files (seeds/character.json,
// weapons.json, items.json, skills.json) for every storage engine: the
// database's seed step and the in-memory store of the e2e mode (ADR 0025 step
// 8.1) load the same values, refused by the same rules. It knows nothing of
// storage: what an engine does with an existing row is its own.
package seedfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// The seed file names, in seedsDir.
const (
	CharacterFile = "character.json"
	WeaponsFile   = "weapons.json"
	ItemsFile     = "items.json"
	SkillsFile    = "skills.json"
)

// Path is file in seedsDir.
func Path(seedsDir, file string) string {
	return filepath.Join(filepath.Clean(seedsDir), file)
}

// Character is seeds/character.json. Level, Thac0Base and the saves are the
// figures the campaign sheet states, kept to compare with the derived values
// (ADR 0030 step 24), never read as inputs. Class and Race are a class and a
// race id of the catalogue's definitions (ADR 0040 step 9: required, checked
// against them by CheckDefinitions); Alignment and Sex are codes
// (ruletablesvc.Alignments, charactersvc.Sexes).
type Character struct {
	Name             string   `json:"name"`
	Class            string   `json:"class"`
	Race             string   `json:"race"`
	Alignment        string   `json:"alignment"`
	Sex              string   `json:"sex"`
	Str              int      `json:"str"`
	Dex              int      `json:"dex"`
	Con              int      `json:"con"`
	Int              int      `json:"int"`
	Wis              int      `json:"wis"`
	Cha              int      `json:"cha"`
	HPMax            int      `json:"hp_max"`
	Thac0Base        int      `json:"thac0_base"`
	Level            int      `json:"level"`
	XPTotal          int      `json:"xp_total"`
	SaveParaPoison   int      `json:"save_para_poison"`
	SaveRodStaffWand int      `json:"save_rod_staff_wand"`
	SavePetriPoly    int      `json:"save_petri_poly"`
	SaveBreath       int      `json:"save_breath"`
	SaveSpell        int      `json:"save_spell"`
	Slots            []any    `json:"slots"`
	Languages        []string `json:"languages"`
	// SkillPoints are the imported bard's skill points (ADR 0030 step 18),
	// by skill code: the points that give the sheet's percentages (step
	// 24), checked as a bard's entered points are.
	SkillPoints map[string]int `json:"skill_points"`
	// Proficiencies are the seed character's non-weapon picks (ADR 0037
	// steps 12 and 16): catalogue id and slots spent, written with the
	// character when it is inserted, never reconciled afterwards (the player
	// edits them). The catalogue check is the seedcheck's.
	Proficiencies []SeedPick `json:"proficiencies"`
	// WeaponProficiencies are the seed character's weapon picks (ADR 0037
	// steps 7 and 16.2): a weapon type and its slot, the types of the seed's
	// weapons, written and never reconciled as Proficiencies are.
	WeaponProficiencies []SeedWeaponPick `json:"weapon_proficiencies"`
}

// SeedPick is one non-weapon pick of the character seed.
type SeedPick struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

// SeedWeaponPick is one weapon pick of the character seed.
type SeedWeaponPick struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

// Picks are the seed's picks as the character stores them.
func (c Character) Picks() skills.Picks {
	out := skills.Picks{
		Nonweapon: make([]skills.Pick, 0, len(c.Proficiencies)),
		Weapon:    make([]skills.WeaponPick, 0, len(c.WeaponProficiencies)),
	}
	for _, p := range c.Proficiencies {
		out.Nonweapon = append(out.Nonweapon, skills.Pick(p))
	}
	for _, p := range c.WeaponProficiencies {
		out.Weapon = append(out.Weapon, skills.WeaponPick(p))
	}
	return out
}

// ReadCharacter reads the character seed at path. ok is false when there is
// nothing to seed: the file is missing or unreadable, or names no character.
func ReadCharacter(path string) (c Character, ok bool, err error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Character{}, false, nil
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Character{}, false, fmt.Errorf("parse character.json: %w", err)
	}
	if c.Name == "" {
		return Character{}, false, nil
	}
	// ADR 0040 step 9: the seed names its class and race.
	switch {
	case c.Class == "":
		return Character{}, false, errors.New("character.json: class is required")
	case c.Race == "":
		return Character{}, false, errors.New("character.json: race is required")
	}
	points, err := skills.ValidSkillPoints(c.Class, c.SkillPoints)
	if err != nil {
		return Character{}, false, fmt.Errorf("character.json: skill points: %w", err)
	}
	c.SkillPoints = points
	seen := map[string]bool{}
	for i, p := range c.Proficiencies {
		switch {
		case p.ID == "":
			return Character{}, false, fmt.Errorf("character.json: proficiency %d: id is required", i)
		case seen[p.ID]:
			return Character{}, false, fmt.Errorf("character.json: proficiency %q twice", p.ID)
		case p.Slots < 1:
			return Character{}, false, fmt.Errorf("character.json: proficiency %q: at least 1 slot", p.ID)
		}
		seen[p.ID] = true
	}
	types := map[string]bool{}
	for i, p := range c.WeaponProficiencies {
		switch {
		case !slices.Contains(ruletablesvc.WeaponTypes, p.Type):
			return Character{}, false, fmt.Errorf("character.json: weapon proficiency %d: unknown weapon type %q", i, p.Type)
		case types[p.Type]:
			return Character{}, false, fmt.Errorf("character.json: weapon proficiency %q twice", p.Type)
		case p.Slots != skills.WeaponPickSlots:
			return Character{}, false, fmt.Errorf("character.json: weapon proficiency %q: must spend %d slot", p.Type, skills.WeaponPickSlots)
		}
		types[p.Type] = true
	}
	return c, true, nil
}

// CheckDefinitions refuses a seed character whose class or race the
// catalogue does not define (ADR 0040 step 9), at start, before anything is
// seeded.
func (c Character) CheckDefinitions(defs *classes.Registry) error {
	if _, ok := defs.Class(c.Class); !ok {
		return fmt.Errorf("character.json: class %q is not one of %v", c.Class, defs.ClassIDs())
	}
	if !slices.Contains(defs.RaceIDs(), c.Race) {
		return fmt.Errorf("character.json: race %q is not one of %v", c.Race, defs.RaceIDs())
	}
	return nil
}

// SlotsJSON is the seed's spell slot list as the spell slots are stored
// ("[]" for none).
func (c Character) SlotsJSON() (string, error) {
	if len(c.Slots) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(c.Slots)
	if err != nil {
		return "", fmt.Errorf("marshal slots: %w", err)
	}
	return string(b), nil
}

// Weapon is one seeds/weapons.json entry; name is a language map (ADR 0023
// step 10).
type Weapon struct {
	ID          string         `json:"id"`
	Name        localized.Text `json:"name"`
	DamageSM    string         `json:"damage_sm"`
	DamageL     string         `json:"damage_l"`
	MagicToHit  int            `json:"magic_to_hit"`
	MagicDamage int            `json:"magic_damage"`
	Kind        string         `json:"kind"`
	TwoHanded   bool           `json:"two_handed"`
	AmmoKind    string         `json:"ammo_kind"`
	WeaponType  string         `json:"weapon_type"`
}

// ErrUnreadable wraps the read error of a seed file that is missing or
// unreadable: an engine whose table is already filled ignores it, an empty
// one refuses to start (its catalog would be empty).
var ErrUnreadable = errors.New("seed file unreadable")

// ReadWeapons reads and checks the weapon seed at path: ids unique, id, name
// and both damages required.
func ReadWeapons(path string) ([]Weapon, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	var weapons []Weapon
	if err := json.Unmarshal(data, &weapons); err != nil {
		return nil, fmt.Errorf("parse weapons.json: %w", err)
	}
	seen := make(map[string]bool, len(weapons))
	for i, w := range weapons {
		if seen[w.ID] {
			return nil, fmt.Errorf("weapons.json entry %d: duplicate id %q", i, w.ID)
		}
		seen[w.ID] = true
		if w.ID == "" || len(w.Name.Trimmed()) == 0 || w.DamageSM == "" || w.DamageL == "" {
			return nil, fmt.Errorf("weapons.json entry %d: id, name, damage_sm and damage_l are required", i)
		}
	}
	return weapons, nil
}

// Item is one seeds/items.json entry: name is a language map (ADR 0023 step
// 10), behaviour is stock, charges or daily, type a free-text label,
// max_count defaults to count (a daily item's refill value).
type Item struct {
	ID         string         `json:"id"`
	Name       localized.Text `json:"name"`
	Type       string         `json:"type"`
	Behaviour  string         `json:"behaviour"`
	MagicBonus int            `json:"magic_bonus"`
	AmmoKind   string         `json:"ammo_kind"`
	Count      int            `json:"count"`
	MaxCount   *int           `json:"max_count"`
	EffectDice string         `json:"effect_dice"`
	EffectSave string         `json:"effect_save"`
	Notes      string         `json:"notes"`
	// Powers are the item's own powers (ADR 0016 steps 9 to 16), in
	// itemsvc's power encoding; absent: none.
	Powers json.RawMessage `json:"powers"`
}

// Max is the item's max count: max_count, else its count.
func (it Item) Max() int {
	if it.MaxCount != nil {
		return *it.MaxCount
	}
	return it.Count
}

// SeededItem is a checked item seed with its powers decoded and in their
// stored encoding.
type SeededItem struct {
	Item
	PowerList []itemsvc.Power
	// PowersJSON is PowerList in itemsvc's stored encoding ("[]" for none).
	PowersJSON string
}

// ReadItems reads and checks the item seed at path: id and name required,
// ids unique, a known behaviour, a count neither negative nor above
// max_count, and powers that pass itemsvc.ValidatePowers (ADR 0016 steps 9
// and 10).
func ReadItems(path string) ([]SeededItem, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse items.json: %w", err)
	}
	seen := make(map[string]bool, len(items))
	out := make([]SeededItem, len(items))
	for i, it := range items {
		if it.ID == "" || len(it.Name.Trimmed()) == 0 {
			return nil, fmt.Errorf("items.json entry %d: id and name are required", i)
		}
		if seen[it.ID] {
			return nil, fmt.Errorf("items.json entry %d: duplicate id %q", i, it.ID)
		}
		seen[it.ID] = true
		switch it.Behaviour {
		case "stock", "charges", "daily":
		default:
			return nil, fmt.Errorf("items.json entry %d (%q): behaviour must be stock, charges or daily, got %q", i, it.ID, it.Behaviour)
		}
		if it.Count < 0 {
			return nil, fmt.Errorf("items.json entry %d (%q): negative count", i, it.ID)
		}
		if it.MaxCount != nil && it.Count > *it.MaxCount {
			return nil, fmt.Errorf("items.json entry %d (%q): count %d exceeds max_count %d", i, it.ID, it.Count, *it.MaxCount)
		}
		powers, encoded, err := seedPowers(it.Powers)
		if err != nil {
			return nil, fmt.Errorf("items.json entry %d (%q): %w", i, it.ID, err)
		}
		out[i] = SeededItem{Item: it, PowerList: powers, PowersJSON: encoded}
	}
	return out, nil
}

// seedPowers checks a seed item's powers (the rules of
// itemsvc.ValidatePowers) and returns them with their stored encoding ("[]"
// for none).
func seedPowers(raw json.RawMessage) ([]itemsvc.Power, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, "[]", nil
	}
	powers, err := itemsvc.DecodePowers(string(raw))
	if err != nil {
		return nil, "", err
	}
	if err := itemsvc.ValidatePowers(powers); err != nil {
		return nil, "", err
	}
	encoded, err := itemsvc.EncodePowers(powers)
	if err != nil {
		return nil, "", err
	}
	return powers, encoded, nil
}

// Skills is seeds/skills.json: the bard skills (percentage) of ADR 0014, the
// imported sheet's figures. The non-weapon proficiencies are picks of the
// character seed (Character.Proficiencies, ADR 0037 step 16).
type Skills struct {
	BardSkills []BardSkill `json:"bard_skills"`
}

// BardSkill is one bard skill of the seed.
type BardSkill struct {
	ID      string `json:"id"`
	Percent int    `json:"percent"`
}

// ReadSkills reads and checks the skill seed at path: ids present and unique
// across both lists, a known ability, a percentage within 0-100.
func ReadSkills(path string) (Skills, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Skills{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	var s Skills
	if err := json.Unmarshal(data, &s); err != nil {
		return Skills{}, fmt.Errorf("parse skills.json: %w", err)
	}
	return s, s.validate()
}

func (s Skills) validate() error {
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

// Equipment is the seed character's starting equipment: the items and weapons seeds converted
// into pieces by the conversion of the existing data (itemsvc.Convert, ADR 0033 step 26), so a
// new database holds what an upgraded one does. The items come first, in their order, then the
// weapons. An item and a weapon of the same id are refused: the seed names each piece once; so
// is a piece the editor would refuse (itemsvc.Item.Validate).
func Equipment(items []SeededItem, weapons []Weapon, catalogue []ruletablesvc.Weapon) (itemsvc.Conversion, error) {
	legacyItems := make([]itemsvc.Item, 0, len(items))
	ids := map[string]bool{}
	for i, it := range items {
		ids[it.ID] = true
		legacyItems = append(legacyItems, itemsvc.Item{
			ID: it.ID, Position: i, Name: it.Name.Trimmed(), Type: it.Type, Behaviour: itemsvc.Behaviour(it.Behaviour),
			MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind, Count: it.Count, MaxCount: it.Max(),
			EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes, Powers: it.PowerList,
		})
	}
	legacyWeapons := make([]itemsvc.LegacyWeapon, 0, len(weapons))
	for i, w := range weapons {
		if ids[w.ID] {
			return itemsvc.Conversion{}, fmt.Errorf("weapon %q: an item of the seed has that id", w.ID)
		}
		legacyWeapons = append(legacyWeapons, itemsvc.LegacyWeapon{
			ID: w.ID, Position: i, Name: w.Name.Trimmed(), DamageSM: w.DamageSM, DamageL: w.DamageL,
			MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: w.Kind, TwoHanded: w.TwoHanded,
			AmmoKind: w.AmmoKind, Type: w.WeaponType,
		})
	}
	conv := itemsvc.Convert(legacyItems, legacyWeapons, catalogue)
	// The seed's pieces are what the editor would accept (a weapon of an unknown kind, an
	// unknown ammunition kind are refused, as the weapons table's CHECK refused the first).
	for _, p := range conv.Pieces {
		if err := p.Validate(); err != nil {
			return itemsvc.Conversion{}, fmt.Errorf("%q: %w", p.ID, err)
		}
	}
	return conv, nil
}

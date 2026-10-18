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

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// The seed file names, in seedsDir.
const (
	CharacterFile = "character.json"
	WeaponsFile   = "weapons.json"
	ItemsFile     = "items.json"
	SkillsFile    = "skills.json"
)

// DefaultClass is the class a seeded character gets: the seed has no class,
// and the existing character is a bard (the characters table's default).
const DefaultClass = "bard"

// Path is file in seedsDir.
func Path(seedsDir, file string) string {
	return filepath.Join(filepath.Clean(seedsDir), file)
}

// Character is seeds/character.json. Level, Thac0Base and the saves are the
// figures the campaign sheet states, kept to compare with the derived values
// (ADR 0030 step 24), never read as inputs. Race is a race code
// (charactersvc.Races), "" for none.
type Character struct {
	Name             string   `json:"name"`
	Race             string   `json:"race"`
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
	if c.Race != "" && !slices.Contains(charactersvc.Races, c.Race) {
		return Character{}, false, fmt.Errorf("character.json: race %q is not one of %v", c.Race, charactersvc.Races)
	}
	return c, c.Name != "", nil
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

// Skills is seeds/skills.json: the non-weapon proficiencies (ability key and
// modifier) and the bard skills (percentage) of ADR 0014. Ids are shared by
// both lists (a check is rolled by id) and named by the UI.
type Skills struct {
	Proficiencies []Proficiency `json:"proficiencies"`
	BardSkills    []BardSkill   `json:"bard_skills"`
}

// Proficiency is one non-weapon proficiency of the seed.
type Proficiency struct {
	ID       string `json:"id"`
	Ability  string `json:"ability"`
	Modifier int    `json:"modifier"`
}

// BardSkill is one bard skill of the seed.
type BardSkill struct {
	ID      string `json:"id"`
	Percent int    `json:"percent"`
}

var abilityKeys = map[string]bool{"str": true, "dex": true, "con": true, "int": true, "wis": true, "cha": true}

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

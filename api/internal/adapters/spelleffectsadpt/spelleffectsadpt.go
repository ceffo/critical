// Package spelleffectsadpt is a spell-effects catalog backed by
// <seedsDir>/spell_effects.json, loaded and validated once at startup.
//
// The seed holds one structured entry per Wizard spell: what kind of effect it
// is (buff, damage or utility), formulas for duration, charges and damage as a
// function of caster level, and a confidence flag. Formulas use the grammar of
// package formula. Later work (buff application, casting) consumes the catalog.
package spelleffectsadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ceffo/critical/api/pkg/formula"
)

// FileName is the seed file looked up inside the seeds directory.
const FileName = "spell_effects.json"

// Kind classifies what a spell does to the combat/buff engine.
type Kind string

// Spell effect kinds.
const (
	KindBuff    Kind = "buff"    // modifies a creature for a while (includes debuffs)
	KindDamage  Kind = "damage"  // deals hit-point damage
	KindUtility Kind = "utility" // nothing the engine tracks numerically
)

// Confidence says how far the extraction can be trusted.
type Confidence string

// Confidence levels. A low entry always carries a note.
const (
	ConfidenceHigh Confidence = "high"
	ConfidenceLow  Confidence = "low"
)

// DurationKind says how a spell's duration is expressed.
type DurationKind string

// Duration kinds.
const (
	DurationRounds          DurationKind = "rounds"           // Rounds formula; 1 turn = 10 rounds, 1 hour = 60
	DurationInstant         DurationKind = "instant"          // effect happens once
	DurationPermanent       DurationKind = "permanent"        // until dispelled or undone
	DurationUntilDischarged DurationKind = "until_discharged" // ends when its charges are used up
	DurationSpecial         DurationKind = "special"          // see the spell text
)

// Target says who a spell affects.
type Target string

// Targets.
const (
	TargetSelf        Target = "self"
	TargetOther       Target = "other"
	TargetSelfOrOther Target = "self_or_other"
	TargetArea        Target = "area"
	TargetNone        Target = "none"
)

// SaveType is the saving throw category.
type SaveType string

// SaveEffect is what a successful save does.
type SaveEffect string

// Save effects.
const (
	SaveHalf    SaveEffect = "half"
	SaveNegates SaveEffect = "negates"
	SaveNone    SaveEffect = "none"
	SaveSpecial SaveEffect = "special"
)

// Duration is a spell duration. Rounds is set only when Kind is DurationRounds.
type Duration struct {
	Kind   DurationKind
	Rounds *formula.Formula
}

// Save is the saving throw a target gets.
type Save struct {
	Type   SaveType
	Effect SaveEffect
}

// Charges is a pool consumed instead of (or besides) time, e.g. Stoneskin attacks.
type Charges struct {
	Count *formula.Formula
	Unit  string
}

// LevelDice is one row of a by-caster-level damage table.
type LevelDice struct {
	MinLevel int
	Dice     *formula.Formula
}

// Damage describes hit-point damage. Exactly one of Dice and ByLevel is set.
type Damage struct {
	Dice    *formula.Formula
	ByLevel []LevelDice // ascending MinLevel; the last row with MinLevel <= level applies
	// Instances is how many separate damage rolls one casting makes (missiles,
	// bolts). Nil means one.
	Instances *formula.Formula
	// Rounds is how many rounds the damage repeats, rolled every round
	// (Melf's acid). Nil means once.
	Rounds *formula.Formula
	Type   string
}

// AbilityGain raises an ability score. Gain applies to everybody; ByGroup picks
// the die by class group (warrior, priest, rogue, wizard; a bard is a rogue).
type AbilityGain struct {
	Gain       *formula.Formula
	ByGroup    map[string]*formula.Formula
	Max        int
	MaxByGroup map[string]int
}

// Buff is the modifier payload of a buff or debuff. Sign conventions are in
// seeds/spell_effects.schema.json.
type Buff struct {
	Modifiers   map[string]*formula.Formula
	AbilityGain map[string]AbilityGain
	AbilitySet  map[string]string
	Condition   string
}

// Entry is one validated spell effect.
type Entry struct {
	ID         string
	Name       string
	Level      int
	Kind       Kind
	Confidence Confidence
	Note       string
	Target     Target
	Duration   Duration
	Charges    *Charges
	Save       *Save
	Damage     *Damage
	Buff       *Buff
}

// Catalog is the loaded set of spell effects, keyed by spell id.
type Catalog struct {
	byID  map[string]Entry
	order []string
}

// Slug is the interim spell id: the snake_case of the English spell name.
// Lowercase, apostrophes dropped, every other run of non-alphanumerics becomes
// one underscore ("Melf's Acid Arrow" -> "melfs_acid_arrow").
func Slug(name string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r == '\'' || r == '’' || r == '*':
			// dropped, does not split words
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
		default:
			pendingSep = true
		}
	}
	return b.String()
}

// New loads and validates <seedsDir>/spell_effects.json. A missing file, a
// malformed formula or any inconsistent entry is an error naming the spell and
// the field.
func New(seedsDir string) (*Catalog, error) {
	path := filepath.Join(seedsDir, FileName)
	raw, err := os.ReadFile(path) //nolint:gosec // path is built from the operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("spell effects: read %s: %w", path, err)
	}
	c, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("spell effects: %s: %w", path, err)
	}
	return c, nil
}

// Parse validates seed JSON bytes into a Catalog.
func Parse(raw []byte) (*Catalog, error) {
	var entries []rawEntry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	c := &Catalog{byID: make(map[string]Entry, len(entries))}
	for i, re := range entries {
		label := fmt.Sprintf("entry %d", i)
		if re.ID != "" {
			label = fmt.Sprintf("spell %q", re.ID)
		}
		e, err := re.validate()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if _, dup := c.byID[e.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate id", label)
		}
		c.byID[e.ID] = e
		c.order = append(c.order, e.ID)
	}
	return c, nil
}

// Get returns the effect by spell id and a presence flag.
func (c *Catalog) Get(id string) (Entry, bool) {
	e, ok := c.byID[id]
	return e, ok
}

// Len is the number of entries.
func (c *Catalog) Len() int { return len(c.order) }

// All returns the entries in file order.
func (c *Catalog) All() []Entry {
	out := make([]Entry, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.byID[id])
	}
	return out
}

// ---- raw JSON shape and validation ----

// flexString accepts a JSON string or number and keeps its text, so the seed
// can write "ac_set": 6 as well as "to_hit": "1d4".
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("want a string or a number, got %s", b)
	}
	*f = flexString(n.String())
	return nil
}

type rawEntry struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Level       int                       `json:"level"`
	Kind        Kind                      `json:"kind"`
	Confidence  Confidence                `json:"confidence"`
	Note        string                    `json:"note"`
	Target      Target                    `json:"target"`
	Duration    rawDuration               `json:"duration"`
	Charges     *rawCharges               `json:"charges"`
	Save        *rawSave                  `json:"save"`
	Damage      *rawDamage                `json:"damage"`
	Modifiers   rawModifiers              `json:"modifiers"`
	AbilityGain map[string]rawAbilityGain `json:"ability_gain"`
	AbilitySet  map[string]flexString     `json:"ability_set"`
	Condition   string                    `json:"condition"`
}

type rawDuration struct {
	Kind   DurationKind `json:"kind"`
	Rounds flexString   `json:"rounds"`
}

type rawCharges struct {
	Count flexString `json:"count"`
	Unit  string     `json:"unit"`
}

type rawSave struct {
	Type   SaveType   `json:"type"`
	Effect SaveEffect `json:"effect"`
}

type rawLevelDice struct {
	MinLevel int        `json:"min_level"`
	Dice     flexString `json:"dice"`
}

type rawDamage struct {
	Dice      flexString     `json:"dice"`
	ByLevel   []rawLevelDice `json:"by_level"`
	Instances flexString     `json:"instances"`
	Rounds    flexString     `json:"rounds"`
	Type      string         `json:"type"`
}

type rawAbilityGain struct {
	Gain       flexString            `json:"gain"`
	ByGroup    map[string]flexString `json:"by_group"`
	Max        int                   `json:"max"`
	MaxByGroup map[string]int        `json:"max_by_group"`
}

type rawModifiers map[string]flexString

var (
	validKinds = []Kind{KindBuff, KindDamage, KindUtility}
	// validSaveTypes mirror the AD&D 2e saving throw categories, plus "none".
	validSaveTypes   = []SaveType{"none", "spell", "poison", "paralyzation", "death", "petrification", "breath", "rod_staff_wand", "polymorph"}
	validSaveEffects = []SaveEffect{SaveHalf, SaveNegates, SaveNone, SaveSpecial}
	validTargets     = []Target{TargetSelf, TargetOther, TargetSelfOrOther, TargetArea, TargetNone}
	validDurations   = []DurationKind{DurationRounds, DurationInstant, DurationPermanent, DurationUntilDischarged, DurationSpecial}
	validAbilities   = []string{"str", "dex", "con", "int", "wis", "cha"}
	validGroups      = []string{"warrior", "priest", "rogue", "wizard"}
	// validModifiers are the scalar buff keys, each a formula.
	validModifiers = []string{
		"to_hit", "damage", "ac", "ac_set", "ac_set_thrown", "ac_set_missile", "ac_set_other",
		"save", "attackers_to_hit", "attackers_to_hit_first", "initiative",
		"move_pct", "attacks_pct", "damage_multiplier_pct", "weapon_buff",
		"regen_hp_per_round",
	}
)

func oneOf[T comparable](v T, set []T, what string) error {
	if !slices.Contains(set, v) {
		return fmt.Errorf("%s %v is not one of %v", what, v, set)
	}
	return nil
}

func parseFormula(field string, s flexString) (*formula.Formula, error) {
	f, err := formula.Parse(string(s))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return f, nil
}

func (r rawEntry) validate() (Entry, error) {
	e := Entry{
		ID: r.ID, Name: r.Name, Level: r.Level, Kind: r.Kind,
		Confidence: r.Confidence, Note: r.Note, Target: r.Target,
	}
	if r.ID == "" || r.Name == "" {
		return e, fmt.Errorf("id and name are required")
	}
	if want := Slug(r.Name); r.ID != want {
		return e, fmt.Errorf("id must be the slug of the name %q (%q)", r.Name, want)
	}
	if r.Level < 1 || r.Level > 9 {
		return e, fmt.Errorf("level %d out of range 1..9", r.Level)
	}
	if err := oneOf(r.Kind, validKinds, "kind"); err != nil {
		return e, err
	}
	if err := oneOf(r.Confidence, []Confidence{ConfidenceHigh, ConfidenceLow}, "confidence"); err != nil {
		return e, err
	}
	if r.Confidence == ConfidenceLow && strings.TrimSpace(r.Note) == "" {
		return e, fmt.Errorf("a low-confidence entry needs a note")
	}
	if err := oneOf(r.Target, validTargets, "target"); err != nil {
		return e, err
	}
	if err := r.validateDuration(&e); err != nil {
		return e, err
	}
	if err := r.validateSave(&e); err != nil {
		return e, err
	}
	if err := r.validateCharges(&e); err != nil {
		return e, err
	}
	hasBuff := len(r.Modifiers) > 0 || len(r.AbilityGain) > 0 || len(r.AbilitySet) > 0
	switch r.Kind {
	case KindBuff:
		if !hasBuff && r.Charges == nil {
			return e, fmt.Errorf("a buff needs modifiers, ability_gain, ability_set or charges")
		}
		if r.Damage != nil {
			return e, fmt.Errorf("a buff cannot have damage")
		}
		return e, r.validateBuff(&e)
	case KindDamage:
		if r.Damage == nil {
			return e, fmt.Errorf("a damage spell needs damage")
		}
		if hasBuff || r.Charges != nil {
			return e, fmt.Errorf("a damage spell cannot have buff fields")
		}
		return e, r.validateDamage(&e)
	default: // utility
		if hasBuff || r.Charges != nil || r.Damage != nil {
			return e, fmt.Errorf("a utility spell cannot have buff, charges or damage fields")
		}
	}
	return e, nil
}

func (r rawEntry) validateDuration(e *Entry) error {
	d := r.Duration
	if err := oneOf(d.Kind, validDurations, "duration.kind"); err != nil {
		return err
	}
	e.Duration.Kind = d.Kind
	if d.Kind == DurationRounds {
		f, err := parseFormula("duration.rounds", d.Rounds)
		if err != nil {
			return err
		}
		e.Duration.Rounds = f
	} else if d.Rounds != "" {
		return fmt.Errorf("duration.rounds is only allowed with kind %q", DurationRounds)
	}
	return nil
}

func (r rawEntry) validateSave(e *Entry) error {
	if r.Save == nil {
		return nil
	}
	if err := oneOf(r.Save.Type, validSaveTypes, "save.type"); err != nil {
		return err
	}
	if err := oneOf(r.Save.Effect, validSaveEffects, "save.effect"); err != nil {
		return err
	}
	e.Save = &Save{Type: r.Save.Type, Effect: r.Save.Effect}
	return nil
}

func (r rawEntry) validateCharges(e *Entry) error {
	if r.Charges == nil {
		return nil
	}
	f, err := parseFormula("charges.count", r.Charges.Count)
	if err != nil {
		return err
	}
	if strings.TrimSpace(r.Charges.Unit) == "" {
		return fmt.Errorf("charges.unit is required")
	}
	e.Charges = &Charges{Count: f, Unit: r.Charges.Unit}
	return nil
}

func (r rawEntry) validateBuff(e *Entry) error {
	b := &Buff{Condition: r.Condition}
	if len(r.Modifiers) > 0 {
		b.Modifiers = make(map[string]*formula.Formula, len(r.Modifiers))
	}
	for _, k := range sortedKeys(r.Modifiers) {
		if err := oneOf(k, validModifiers, "modifiers key"); err != nil {
			return err
		}
		f, err := parseFormula("modifiers."+k, r.Modifiers[k])
		if err != nil {
			return err
		}
		b.Modifiers[k] = f
	}
	for _, ab := range sortedKeys(r.AbilityGain) {
		if err := oneOf(ab, validAbilities, "ability_gain key"); err != nil {
			return err
		}
		g, err := r.AbilityGain[ab].validate("ability_gain." + ab)
		if err != nil {
			return err
		}
		if b.AbilityGain == nil {
			b.AbilityGain = map[string]AbilityGain{}
		}
		b.AbilityGain[ab] = g
	}
	for _, ab := range sortedKeys(r.AbilitySet) {
		if err := oneOf(ab, validAbilities, "ability_set key"); err != nil {
			return err
		}
		if strings.TrimSpace(string(r.AbilitySet[ab])) == "" {
			return fmt.Errorf("ability_set.%s is empty", ab)
		}
		if b.AbilitySet == nil {
			b.AbilitySet = map[string]string{}
		}
		b.AbilitySet[ab] = string(r.AbilitySet[ab])
	}
	e.Buff = b
	return nil
}

func (g rawAbilityGain) validate(field string) (AbilityGain, error) {
	out := AbilityGain{Max: g.Max, MaxByGroup: g.MaxByGroup}
	if (g.Gain == "") == (len(g.ByGroup) == 0) {
		return out, fmt.Errorf("%s: set exactly one of gain and by_group", field)
	}
	if g.Gain != "" {
		f, err := parseFormula(field+".gain", g.Gain)
		if err != nil {
			return out, err
		}
		out.Gain = f
	}
	if len(g.ByGroup) > 0 {
		out.ByGroup = make(map[string]*formula.Formula, len(g.ByGroup))
	}
	for _, grp := range sortedKeys(g.ByGroup) {
		if err := oneOf(grp, validGroups, field+".by_group key"); err != nil {
			return out, err
		}
		f, err := parseFormula(field+".by_group."+grp, g.ByGroup[grp])
		if err != nil {
			return out, err
		}
		out.ByGroup[grp] = f
	}
	for grp, m := range g.MaxByGroup {
		if err := oneOf(grp, validGroups, field+".max_by_group key"); err != nil {
			return out, err
		}
		if m < 1 || m > 30 {
			return out, fmt.Errorf("%s.max_by_group.%s: %d out of range", field, grp, m)
		}
	}
	if g.Max < 0 || g.Max > 30 {
		return out, fmt.Errorf("%s.max: %d out of range", field, g.Max)
	}
	return out, nil
}

func (r rawEntry) validateDamage(e *Entry) error {
	d := r.Damage
	out := &Damage{Type: d.Type}
	if (d.Dice == "") == (len(d.ByLevel) == 0) {
		return fmt.Errorf("damage: set exactly one of dice and by_level")
	}
	if d.Dice != "" {
		f, err := parseFormula("damage.dice", d.Dice)
		if err != nil {
			return err
		}
		out.Dice = f
	}
	prev := 0
	for i, row := range d.ByLevel {
		if row.MinLevel <= prev {
			return fmt.Errorf("damage.by_level[%d]: min_level %d must be above the previous row", i, row.MinLevel)
		}
		prev = row.MinLevel
		f, err := parseFormula(fmt.Sprintf("damage.by_level[%d].dice", i), row.Dice)
		if err != nil {
			return err
		}
		out.ByLevel = append(out.ByLevel, LevelDice{MinLevel: row.MinLevel, Dice: f})
	}
	if len(out.ByLevel) > 0 && out.ByLevel[0].MinLevel != 1 {
		return fmt.Errorf("damage.by_level must start at min_level 1")
	}
	var err error
	if d.Instances != "" {
		if out.Instances, err = parseFormula("damage.instances", d.Instances); err != nil {
			return err
		}
	}
	if d.Rounds != "" {
		if out.Rounds, err = parseFormula("damage.rounds", d.Rounds); err != nil {
			return err
		}
	}
	e.Damage = out
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// String describes the duration for error messages and logs.
func (d Duration) String() string {
	if d.Kind == DurationRounds {
		return d.Rounds.String() + " rounds"
	}
	return string(d.Kind)
}

// Package spelleffectsadpt implements spelleffectsvc.Catalog from
// <seedsDir>/spell_effects.json, loaded and validated once at startup. It only
// holds private JSON DTOs and the mapping to the domain types; the entities
// and their rules live in spelleffectsvc.
package spelleffectsadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

// FileName is the seed file looked up inside the seeds directory.
const FileName = "spell_effects.json"

// Catalog satisfies spelleffectsvc.Catalog.
type Catalog struct {
	byID  map[string]spelleffectsvc.Entry
	order []string
}

var _ spelleffectsvc.Catalog = (*Catalog)(nil)

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

// Parse maps seed JSON bytes to a validated Catalog.
func Parse(raw []byte) (*Catalog, error) {
	var entries []entryDTO
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	c := &Catalog{byID: make(map[string]spelleffectsvc.Entry, len(entries))}
	for i, dto := range entries {
		label := fmt.Sprintf("entry %d", i)
		if dto.ID != "" {
			label = fmt.Sprintf("spell %q", dto.ID)
		}
		e, err := dto.toDomain()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if err := e.Validate(); err != nil {
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
func (c *Catalog) Get(id string) (spelleffectsvc.Entry, bool) {
	e, ok := c.byID[id]
	return e, ok
}

// Len is the number of entries.
func (c *Catalog) Len() int { return len(c.order) }

// All returns the entries in file order.
func (c *Catalog) All() []spelleffectsvc.Entry {
	out := make([]spelleffectsvc.Entry, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.byID[id])
	}
	return out
}

// ---- JSON DTOs ----

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

type entryDTO struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Level       int                       `json:"level"`
	Kind        string                    `json:"kind"`
	Confidence  string                    `json:"confidence"`
	Note        string                    `json:"note"`
	Target      string                    `json:"target"`
	Duration    durationDTO               `json:"duration"`
	Charges     *chargesDTO               `json:"charges"`
	Save        *saveDTO                  `json:"save"`
	Damage      *damageDTO                `json:"damage"`
	Modifiers   map[string]flexString     `json:"modifiers"`
	AbilityGain map[string]abilityGainDTO `json:"ability_gain"`
	AbilitySet  map[string]flexString     `json:"ability_set"`
	Condition   string                    `json:"condition"`
	GrantSlots  *grantSlotsDTO            `json:"grant_slots"`
	ConsumeItem *consumeItemDTO           `json:"consume_item"`
	AttackRoll  bool                      `json:"attack_roll"`
}

type grantSlotsDTO struct {
	Levels int `json:"levels"`
}

type consumeItemDTO struct {
	Type string `json:"type"`
}

type durationDTO struct {
	Kind   string     `json:"kind"`
	Rounds flexString `json:"rounds"`
}

type chargesDTO struct {
	Count flexString `json:"count"`
	Unit  string     `json:"unit"`
}

type saveDTO struct {
	Type   string `json:"type"`
	Effect string `json:"effect"`
}

type levelDiceDTO struct {
	MinLevel int        `json:"min_level"`
	Dice     flexString `json:"dice"`
}

type damageDTO struct {
	Dice      flexString     `json:"dice"`
	ByLevel   []levelDiceDTO `json:"by_level"`
	Parts     []partDTO      `json:"parts"`
	Instances flexString     `json:"instances"`
	Rounds    flexString     `json:"rounds"`
	Type      string         `json:"type"`
}

// partDTO is one damage part; Save is a save effect ("" takes the entry's).
type partDTO struct {
	Dice    flexString     `json:"dice"`
	ByLevel []levelDiceDTO `json:"by_level"`
	Save    string         `json:"save"`
}

type abilityGainDTO struct {
	Gain       flexString            `json:"gain"`
	ByGroup    map[string]flexString `json:"by_group"`
	Max        int                   `json:"max"`
	MaxByGroup map[string]int        `json:"max_by_group"`
}

// ---- mapping ----

// parseFormula parses a required formula; field names the JSON path in errors.
func parseFormula(field string, s flexString) (*formula.Formula, error) {
	f, err := formula.Parse(string(s))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return f, nil
}

// optFormula parses a formula that may be absent (empty string -> nil).
func optFormula(field string, s flexString) (*formula.Formula, error) {
	if s == "" {
		return nil, nil
	}
	return parseFormula(field, s)
}

func (d entryDTO) toDomain() (spelleffectsvc.Entry, error) {
	e := spelleffectsvc.Entry{
		ID: d.ID, Name: d.Name, Level: d.Level,
		Kind:       spelleffectsvc.Kind(d.Kind),
		Confidence: spelleffectsvc.Confidence(d.Confidence),
		Note:       d.Note,
		Target:     spelleffectsvc.Target(d.Target),
		Duration:   spelleffectsvc.Duration{Kind: spelleffectsvc.DurationKind(d.Duration.Kind)},
		AttackRoll: d.AttackRoll,
	}
	if d.GrantSlots != nil {
		e.GrantSlots = &spelleffectsvc.GrantSlots{Levels: d.GrantSlots.Levels}
	}
	if d.ConsumeItem != nil {
		e.ConsumeItem = &spelleffectsvc.ConsumeItem{Type: d.ConsumeItem.Type}
	}
	var err error
	// A rounds-kind duration with no formula must fail on the formula, not be skipped.
	if d.Duration.Kind == string(spelleffectsvc.DurationRounds) {
		e.Duration.Rounds, err = parseFormula("duration.rounds", d.Duration.Rounds)
	} else {
		e.Duration.Rounds, err = optFormula("duration.rounds", d.Duration.Rounds)
	}
	if err != nil {
		return e, err
	}
	if d.Save != nil {
		e.Save = &spelleffectsvc.Save{Type: spelleffectsvc.SaveType(d.Save.Type), Effect: spelleffectsvc.SaveEffect(d.Save.Effect)}
	}
	if d.Charges != nil {
		count, err := parseFormula("charges.count", d.Charges.Count)
		if err != nil {
			return e, err
		}
		e.Charges = &spelleffectsvc.Charges{Count: count, Unit: d.Charges.Unit}
	}
	if d.Damage != nil {
		if e.Damage, err = d.Damage.toDomain(); err != nil {
			return e, err
		}
	}
	if e.Buff, err = d.buffToDomain(); err != nil {
		return e, err
	}
	return e, nil
}

func (d entryDTO) buffToDomain() (*spelleffectsvc.Buff, error) {
	if len(d.Modifiers) == 0 && len(d.AbilityGain) == 0 && len(d.AbilitySet) == 0 && d.Condition == "" {
		return nil, nil
	}
	b := &spelleffectsvc.Buff{Condition: d.Condition}
	if len(d.Modifiers) > 0 {
		b.Modifiers = make(map[string]*formula.Formula, len(d.Modifiers))
	}
	for _, k := range sortedKeys(d.Modifiers) {
		f, err := parseFormula("modifiers."+k, d.Modifiers[k])
		if err != nil {
			return nil, err
		}
		b.Modifiers[k] = f
	}
	for _, ab := range sortedKeys(d.AbilityGain) {
		g, err := d.AbilityGain[ab].toDomain("ability_gain." + ab)
		if err != nil {
			return nil, err
		}
		if b.AbilityGain == nil {
			b.AbilityGain = map[string]spelleffectsvc.AbilityGain{}
		}
		b.AbilityGain[ab] = g
	}
	for ab, v := range d.AbilitySet {
		if b.AbilitySet == nil {
			b.AbilitySet = map[string]string{}
		}
		b.AbilitySet[ab] = string(v)
	}
	return b, nil
}

func (g abilityGainDTO) toDomain(field string) (spelleffectsvc.AbilityGain, error) {
	out := spelleffectsvc.AbilityGain{Max: g.Max, MaxByGroup: g.MaxByGroup}
	var err error
	if out.Gain, err = optFormula(field+".gain", g.Gain); err != nil {
		return out, err
	}
	if len(g.ByGroup) > 0 {
		out.ByGroup = make(map[string]*formula.Formula, len(g.ByGroup))
	}
	for _, grp := range sortedKeys(g.ByGroup) {
		if out.ByGroup[grp], err = parseFormula(field+".by_group."+grp, g.ByGroup[grp]); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (d damageDTO) toDomain() (*spelleffectsvc.Damage, error) {
	out := &spelleffectsvc.Damage{Type: d.Type}
	var err error
	if out.Dice, out.ByLevel, err = diceToDomain("damage", d.Dice, d.ByLevel); err != nil {
		return nil, err
	}
	// An empty "parts": [] stays an empty, non-nil list: validation refuses it.
	if d.Parts != nil {
		out.Parts = make([]spelleffectsvc.DamagePart, 0, len(d.Parts))
	}
	for i, p := range d.Parts {
		part := spelleffectsvc.DamagePart{Save: spelleffectsvc.SaveEffect(p.Save)}
		if part.Dice, part.ByLevel, err = diceToDomain(fmt.Sprintf("damage.parts[%d]", i), p.Dice, p.ByLevel); err != nil {
			return nil, err
		}
		out.Parts = append(out.Parts, part)
	}
	if out.Instances, err = optFormula("damage.instances", d.Instances); err != nil {
		return nil, err
	}
	if out.Rounds, err = optFormula("damage.rounds", d.Rounds); err != nil {
		return nil, err
	}
	return out, nil
}

// diceToDomain parses an optional dice formula and a by-level table; field
// names the JSON path in errors.
func diceToDomain(field string, dice flexString, byLevel []levelDiceDTO) (*formula.Formula, []spelleffectsvc.LevelDice, error) {
	f, err := optFormula(field+".dice", dice)
	if err != nil {
		return nil, nil, err
	}
	var rows []spelleffectsvc.LevelDice
	for i, row := range byLevel {
		rf, err := parseFormula(fmt.Sprintf("%s.by_level[%d].dice", field, i), row.Dice)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, spelleffectsvc.LevelDice{MinLevel: row.MinLevel, Dice: rf})
	}
	return f, rows, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

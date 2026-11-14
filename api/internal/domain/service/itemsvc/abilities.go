package itemsvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// LimitKind is how often an ability may be used (ADR 0049 step 5).
type LimitKind string

// The usage limits, as stored and sent on the wire.
const (
	// LimitUnlimited is never spent.
	LimitUnlimited LimitKind = "unlimited"
	// LimitPerDay is Max uses a day, Left of them still to use; the long
	// rest refills it (a later slice).
	LimitPerDay LimitKind = "per_day"
	// LimitCharges is a total of Max charges held, Left of them still there,
	// each use costing Cost; never recharged by the app.
	LimitCharges LimitKind = "charges"
	// LimitWithItem spends one of the item's quantity per use.
	LimitWithItem LimitKind = "with_item"
)

// Bounds of the limit's numbers (ADR 0049 step 5).
const (
	// MaxPerDay is the highest daily number.
	MaxPerDay = 99
	// MaxCharges is the highest number of charges an ability holds.
	MaxCharges = 100
	// MaxAbilityNote bounds an ability's note, as an item's notes.
	MaxAbilityNote = 2000
	// MaxAbilities is the most abilities an item holds; the list is copied into
	// every snapshot and log entry, so it is bounded.
	MaxAbilities = 20
	// MaxAbilityEffects is the most effects one ability holds.
	MaxAbilityEffects = 10
	// MaxAbilityName bounds each language's text of an ability's name.
	MaxAbilityName = 100
	// MaxEffectLabel bounds a dice effect's label.
	MaxEffectLabel = 100
)

// Limit is an ability's usage limit (ADR 0049 step 5). Max, Left and Cost
// are set only for the kinds that have them (per_day: Max, Left; charges:
// Max, Left, Cost); unlimited and with_item hold none. The stored Left never
// exceeds Max.
type Limit struct {
	Kind LimitKind
	Max  int
	Left int
	Cost int
}

// EffectKind is the kind of an ability's effect (ADR 0049 step 10's closed
// list). dice, heal and buff exist so far; spell comes with its own slice.
type EffectKind string

const (
	// EffectDice is dice rolled, logged and shown, applied to nothing (ADR
	// 0049 step 9: the app tracks no monster's hit points).
	EffectDice EffectKind = "dice"
	// EffectHeal is hit points regained by the character through the HP ledger
	// (ADR 0049 steps 9 and 12): Dice, or Full for the gap to max HP.
	EffectHeal EffectKind = "heal"
	// EffectBuff is a timed buff put on the character's list through the buff
	// service, as a hand-made one (ADR 0049 steps 9 and 12): Name, Rounds and
	// the modifiers in Buff.
	EffectBuff EffectKind = "buff"
	// EffectSpell is a spell cast at a fixed level through the cast service
	// (ADR 0049 steps 9, 10 and 12): SpellID and Level; no slot, no
	// preparation.
	EffectSpell EffectKind = "spell"
)

// MaxBuffRounds is the most rounds a buff effect lasts (ADR 0049 step 10: a
// number of rounds, 0 untimed as ADR 0004 step 2): a potion of a day is 144
// rounds an hour, so a day is well under this.
const MaxBuffRounds = 10000

// MaxBuffName bounds a buff effect's name, as an ability's.
const MaxBuffName = 100

// Effect is one effect of an ability. A dice effect holds Dice ("NdM" or
// "NdM+K"), an optional save category (Save) and an optional Label. A heal
// holds Dice or Full (never both) and an optional Label, no save. A buff
// holds Name (empty: the ability's, else the item's), Rounds (0: untimed) and
// the modifiers in Buff (the buffs' effects of ADR 0004 step 11, a Strength
// one allowed); no dice, save or label.
type Effect struct {
	Kind   EffectKind
	Dice   string
	Full   bool
	Save   string
	Label  string
	Name   string
	Rounds int
	Buff   buffsvc.BuffEffects
	// SpellID and Level are a spell effect's spell (a catalogue id) and the
	// caster level it is cast at, 1 to MaxSpellEffectLevel.
	SpellID string
	Level   int
}

// Ability is one of an item's abilities (ADR 0049 step 4): something the
// item does when used, under a usage limit. ID is a slug unique within the
// item; Name (a language map, ADR 0023) empty means the item's own name;
// Effects empty is a use that only spends the limit and shows the note.
type Ability struct {
	ID      string
	Name    localized.Text
	Note    string
	Limit   Limit
	Effects []Effect
}

var abilityIDRE = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// PaysPerObject reports whether the limit is one counter per object (per_day
// and charges): the item may carry such an ability only while it is one
// object, never more than one (ADR 0049 step 4; a one-object piece spent to 0
// keeps it, so it stays editable).
func (l Limit) PaysPerObject() bool { return l.Kind == LimitPerDay || l.Kind == LimitCharges }

// OneCounterPerObject reports whether one of the abilities is a per_day or
// charges counter: the item is one object, and its count is its quantity, 1
// (ADR 0049 step 2).
func OneCounterPerObject(abilities []Ability) bool {
	return slices.ContainsFunc(abilities, func(a Ability) bool { return a.Limit.PaysPerObject() })
}

// HasStock reports whether the piece's quantity is a stock of something
// (ADR 0049 step 2): ammunition, gear, a potion or a scroll. A with_item
// ability spends from it.
func (i Item) HasStock() bool {
	switch i.Category {
	case CategoryAmmunition, CategoryGear:
		return true
	case CategoryMagic:
		return i.MagicKind == MagicPotion || i.MagicKind == MagicScroll
	}
	return false
}

// validAbilities checks the piece's abilities (ADR 0049 steps 4, 5 and 10):
// field "abilities", one refusal message per rule. None (nil, not converted
// yet) and an empty list are fine.
func (i Item) validAbilities() error {
	if len(i.Abilities) > MaxAbilities {
		return domain.FieldErrorf("abilities", "An item holds at most %d abilities", MaxAbilities)
	}
	seen := make(map[string]bool, len(i.Abilities))
	for n, a := range i.Abilities {
		fail := func(format string, args ...any) error {
			return domain.FieldErrorf("abilities", "Ability %d: "+format, append([]any{n + 1}, args...)...)
		}
		if !abilityIDRE.MatchString(a.ID) {
			return fail("the id must be a slug (a-z, 0-9, _)")
		}
		if seen[a.ID] {
			return fail("duplicate id %q", a.ID)
		}
		seen[a.ID] = true
		if lang, bad := a.Name.UnknownLanguage(); bad {
			return fail("unknown language %q in the name", lang)
		}
		for lang, text := range a.Name {
			if utf8.RuneCountInString(text) > MaxAbilityName {
				return fail("the name (%s) must be at most %d characters", lang, MaxAbilityName)
			}
		}
		if len(a.Effects) > MaxAbilityEffects {
			return fail("at most %d effects", MaxAbilityEffects)
		}
		if len(a.Note) > MaxAbilityNote {
			return fail("the note must be at most %d characters", MaxAbilityNote)
		}
		if err := a.Limit.valid(); err != nil {
			return fail("%s", err.Error())
		}
		if a.Limit.Kind == LimitWithItem && !i.HasStock() {
			return fail("a with_item ability needs an item that has a quantity (ammunition, gear, a potion or a scroll)")
		}
		for e, ef := range a.Effects {
			if err := ef.valid(); err != nil {
				return fail("effect %d: %s", e+1, err.Error())
			}
		}
	}
	return nil
}

// validAbilityCounts checks the abilities a request adds or changes against
// the piece's quantity (ADR 0049 step 4: per_day and charges are one counter
// per object). It runs only when the request carries abilities: a stored,
// already converted piece (the boot conversion keeps a wand's count as the
// old release stored it, slice note) is kept as it is and stays editable.
func (i Item) validAbilityCounts() error {
	for n, a := range i.Abilities {
		if a.Limit.PaysPerObject() && i.Count > 1 {
			return domain.FieldErrorf("abilities", "Ability %d: a %s ability needs a quantity of 1 at most (one counter per object)", n+1, a.Limit.Kind)
		}
	}
	return nil
}

// valid checks one limit (ADR 0049 step 5).
func (l Limit) valid() error {
	switch l.Kind {
	case LimitUnlimited, LimitWithItem:
		if l.Max != 0 || l.Left != 0 || l.Cost != 0 {
			return fmt.Errorf("a %s limit has no max, left or cost", l.Kind)
		}
	case LimitPerDay:
		if l.Cost != 0 {
			return fmt.Errorf("a per_day limit has no cost")
		}
		return l.validCounter(MaxPerDay)
	case LimitCharges:
		if err := l.validCounter(MaxCharges); err != nil {
			return err
		}
		if l.Cost < 1 || l.Cost > l.Max {
			return fmt.Errorf("the cost must be between 1 and the max (%d)", l.Max)
		}
	default:
		return fmt.Errorf("unknown limit %q (unlimited, per_day, charges or with_item)", l.Kind)
	}
	return nil
}

// validCounter checks max in 1..ceiling and left in 0..max.
func (l Limit) validCounter(ceiling int) error {
	if l.Max < 1 || l.Max > ceiling {
		return fmt.Errorf("the max must be between 1 and %d", ceiling)
	}
	if l.Left < 0 || l.Left > l.Max {
		return fmt.Errorf("the left must be between 0 and the max (%d)", l.Max)
	}
	return nil
}

// valid checks one effect (ADR 0049 step 10).
func (e Effect) valid() error {
	switch e.Kind {
	case EffectDice:
		if e.Full {
			return fmt.Errorf("a dice effect has no full")
		}
		if _, _, _, err := ParseDice(e.Dice); err != nil {
			return fmt.Errorf("the dice must look like 2d6 or 1d8+2")
		}
	case EffectHeal:
		// ADR 0049 step 10: a heal is dice or full, not both, and no save.
		if e.Save != "" {
			return fmt.Errorf("a heal has no save")
		}
		if e.Full == (e.Dice != "") {
			return fmt.Errorf("a heal is dice or full, not both and not neither")
		}
		if e.Dice != "" {
			if _, _, _, err := ParseDice(e.Dice); err != nil {
				return fmt.Errorf("the dice must look like 2d6 or 1d8+2")
			}
		}
	case EffectBuff:
		// ADR 0049 step 10: a buff carries a name, rounds and the buffs'
		// effects (ADR 0004's rules), and nothing a dice effect holds.
		if e.Dice != "" || e.Full || e.Save != "" || e.Label != "" {
			return fmt.Errorf("a buff has a name, rounds and effects only")
		}
		if utf8.RuneCountInString(e.Name) > MaxBuffName {
			return fmt.Errorf("the buff name must be at most %d characters", MaxBuffName)
		}
		if e.Rounds < 0 || e.Rounds > MaxBuffRounds {
			return fmt.Errorf("the rounds must be between 0 and %d", MaxBuffRounds)
		}
		if err := buffsvc.ValidateEffects(e.Buff); err != nil {
			return errors.New(strings.TrimPrefix(err.Error(), domain.ErrInvalidInput.Error()+": "))
		}
		return nil
	case EffectSpell:
		// ADR 0049 step 10: a spell effect names a spell and a level, and
		// nothing else (whether the catalogue knows the spell is the
		// service's check).
		if e.Dice != "" || e.Full || e.Save != "" || e.Label != "" || e.Name != "" || e.Rounds != 0 || e.Buff != (buffsvc.BuffEffects{}) {
			return fmt.Errorf("a spell has a spell and a level only")
		}
		if !abilityIDRE.MatchString(e.SpellID) {
			return fmt.Errorf("the spell must be a catalogue id")
		}
		if e.Level < 1 || e.Level > MaxSpellEffectLevel {
			return fmt.Errorf("the level must be between 1 and %d", MaxSpellEffectLevel)
		}
		return nil
	default:
		return fmt.Errorf("unknown effect %q (dice, heal, buff or spell)", e.Kind)
	}
	if utf8.RuneCountInString(e.Label) > MaxEffectLabel {
		return fmt.Errorf("the label must be at most %d characters", MaxEffectLabel)
	}
	if e.Save != "" && !slices.Contains(saves.Categories, e.Save) {
		return fmt.Errorf("unknown save category %q", e.Save)
	}
	return nil
}

// withAbilityIDs normalizes the abilities the editor sent: an ability
// without an id gets the first free ability_<n> (n from 1), a charges limit
// without a cost costs 1 (ADR 0049 step 5's default), the typed text and the
// dice are trimmed and lower-cased as an item's. nil stays nil (not
// converted) and an empty list stays empty.
func withAbilityIDs(abilities []Ability) []Ability {
	if abilities == nil {
		return nil
	}
	out := make([]Ability, len(abilities))
	taken := map[string]bool{}
	for n, a := range abilities {
		out[n] = a
		taken[a.ID] = true
	}
	next := 1
	for n := range out {
		a := &out[n]
		if a.ID == "" {
			for taken["ability_"+strconv.Itoa(next)] {
				next++
			}
			a.ID = "ability_" + strconv.Itoa(next)
			taken[a.ID] = true
		}
		a.Name = a.Name.Trimmed()
		a.Note = strings.TrimSpace(a.Note)
		if a.Limit.Kind == LimitCharges && a.Limit.Cost == 0 {
			a.Limit.Cost = 1
		}
		a.Effects = slices.Clone(a.Effects)
		for e := range a.Effects {
			a.Effects[e].Dice = strings.ToLower(strings.TrimSpace(a.Effects[e].Dice))
			a.Effects[e].Save = strings.ToLower(strings.TrimSpace(a.Effects[e].Save))
			a.Effects[e].Label = strings.TrimSpace(a.Effects[e].Label)
			a.Effects[e].Name = strings.TrimSpace(a.Effects[e].Name)
			a.Effects[e].SpellID = strings.ToLower(strings.TrimSpace(a.Effects[e].SpellID))
		}
	}
	return out
}

// jsonAbility is the one encoding of an ability: the items.abilities column
// and the items:<id> snapshots. Add fields, never rename.
type jsonAbility struct {
	ID      string         `json:"id"`
	Name    localized.Text `json:"name,omitempty"`
	Note    string         `json:"note,omitempty"`
	Limit   jsonLimit      `json:"limit"`
	Effects []jsonEffect   `json:"effects"`
}

type jsonLimit struct {
	Kind LimitKind `json:"kind"`
	Max  int       `json:"max,omitempty"`
	Left int       `json:"left,omitempty"`
	Cost int       `json:"cost,omitempty"`
}

type jsonEffect struct {
	Kind  EffectKind `json:"kind"`
	Dice  string     `json:"dice,omitempty"`
	Full  bool       `json:"full,omitempty"`
	Save  string     `json:"save,omitempty"`
	Label string     `json:"label,omitempty"`
	// A buff's name, rounds and modifiers (the buffs' wire keys, as a power's).
	Name   string       `json:"name,omitempty"`
	Rounds int          `json:"rounds,omitempty"`
	Buff   *jsonEffects `json:"buff,omitempty"`
	// A spell effect's spell and caster level.
	SpellID string `json:"spellId,omitempty"`
	Level   int    `json:"level,omitempty"`
}

func toJSONAbilities(abilities []Ability) []jsonAbility {
	out := make([]jsonAbility, 0, len(abilities))
	for _, a := range abilities {
		effects := make([]jsonEffect, 0, len(a.Effects))
		for _, e := range a.Effects {
			j := jsonEffect{Kind: e.Kind, Dice: e.Dice, Full: e.Full, Save: e.Save, Label: e.Label, Name: e.Name, Rounds: e.Rounds, SpellID: e.SpellID, Level: e.Level}
			if e.Kind == EffectBuff {
				b := jsonEffects(e.Buff)
				j.Buff = &b
			}
			effects = append(effects, j)
		}
		out = append(out, jsonAbility{
			ID: a.ID, Name: a.Name, Note: a.Note, Limit: jsonLimit(a.Limit), Effects: effects,
		})
	}
	return out
}

// fromJSONAbilities decodes a list; an empty one is empty, not nil (nil is
// "not converted", ADR 0049 step 18).
func fromJSONAbilities(js []jsonAbility) []Ability {
	out := make([]Ability, 0, len(js))
	for _, j := range js {
		a := Ability{ID: j.ID, Name: j.Name, Note: j.Note, Limit: Limit(j.Limit)}
		for _, e := range j.Effects {
			ef := Effect{Kind: e.Kind, Dice: e.Dice, Full: e.Full, Save: e.Save, Label: e.Label, Name: e.Name, Rounds: e.Rounds, SpellID: e.SpellID, Level: e.Level}
			if e.Buff != nil {
				ef.Buff = buffsvc.BuffEffects(*e.Buff)
			}
			a.Effects = append(a.Effects, ef)
		}
		out = append(out, a)
	}
	return out
}

// EncodeAbilities is the stored JSON of an ability list; ok is false for nil
// (not converted: the column stays NULL, ADR 0049 step 18).
func EncodeAbilities(abilities []Ability) (text string, ok bool, err error) {
	if abilities == nil {
		return "", false, nil
	}
	b, err := json.Marshal(toJSONAbilities(abilities))
	if err != nil {
		return "", false, fmt.Errorf("encode abilities: %w", err)
	}
	return string(b), true, nil
}

// DecodeAbilities reads a stored list: ok false (NULL) is nil, not converted
// yet; "[]" is converted with none. Unknown keys are ignored: a row written
// by a later slice must still read after a rollback (writes stay strict, the
// wire refuses unknown keys).
func DecodeAbilities(raw string, ok bool) ([]Ability, error) {
	if !ok {
		return nil, nil
	}
	var js []jsonAbility
	if err := json.Unmarshal([]byte(raw), &js); err != nil {
		return nil, fmt.Errorf("decode abilities: %w", err)
	}
	return fromJSONAbilities(js), nil
}

// abilitiesText is an ability list as an item_edit change value: its stored
// JSON encoding, "" for not converted (ADR 0049 step 4), which the web words.
func abilitiesText(abilities []Ability) string {
	text, _, err := EncodeAbilities(abilities)
	if err != nil {
		return ""
	}
	return text
}

// ParseAbilities reads an ability list written by hand (the item seed): the
// stored encoding, strict (an unknown key is refused, a typo would otherwise
// drop an effect silently). The abilities are checked with the item they
// belong to (Item.Validate).
func ParseAbilities(raw []byte) ([]Ability, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var js []jsonAbility
	if err := dec.Decode(&js); err != nil {
		return nil, fmt.Errorf("abilities: %w", err)
	}
	return fromJSONAbilities(js), nil
}

// CategoryRule says whether a category has a quantity the player sets (ADR
// 0049 step 2). For the magic category the answer depends on the kind, so
// Kinds lists each kind's answer; for any other category Kinds is empty.
type CategoryRule struct {
	Category    Category
	HasQuantity bool
	Kinds       []KindRule
}

// KindRule is one magic kind's answer.
type KindRule struct {
	Kind        string
	HasQuantity bool
}

// QuantityRules is the rule behind Item.HasStock for every category and magic
// kind, in display order, so the editor can ask it for a piece that is not
// stored yet. Built from HasStock itself: one source.
func QuantityRules() []CategoryRule {
	out := make([]CategoryRule, 0, len(Categories))
	for _, c := range Categories {
		r := CategoryRule{Category: c, HasQuantity: Item{Category: c}.HasStock()}
		if c == CategoryMagic {
			for _, k := range MagicKinds {
				r.Kinds = append(r.Kinds, KindRule{Kind: k, HasQuantity: Item{Category: c, MagicKind: k}.HasStock()})
			}
		}
		out = append(out, r)
	}
	return out
}

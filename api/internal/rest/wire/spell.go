package wire

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/domain/spellvalues"
)

// Text is t as served: a text in human languages (ADR 0023 step 6), body by
// language code, only the languages the catalogue has, and "{}" (never null)
// for a text in none, such as an absent sphere. The client picks its
// language and falls back to "en" per field. localized.Text is the one
// localized-text shape on the wire.
func Text(t localized.Text) localized.Text {
	out := make(localized.Text, len(t))
	for lang, body := range t {
		out[lang] = body
	}
	return out
}

// SpellSlotDTO matches the previous wire shape.
type SpellSlotDTO struct {
	Level             int    `json:"level"`
	PreparedSpellID   string `json:"prepared_spell_id"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
	// Extra: a slot granted by a spell (grant_slots), blank until cast, gone at the long rest.
	Extra bool `json:"extra,omitempty"`
}

// SpellDTO is the wire shape of a spell. Every human-language text is a
// language map (localized.Text, never null: see Text; ADR 0023 step 6): the client picks its language and falls
// back to English per field. The six stat fields keep their raw PHB text
// (English only); Stats is their typed form and Values the computed values
// at CasterLevel (domain types carry the JSON tags). Descriptions are in the
// catalogue's Markdown subset (paragraphs, bullet lists, pipe tables).
type SpellDTO struct {
	// ID is the spell's catalogue key; cast and receive take it.
	ID           string         `json:"id"`
	Name         localized.Text `json:"name"`
	Class        string         `json:"class"`
	Level        int            `json:"level"`
	School       localized.Text `json:"school"`
	Sphere       localized.Text `json:"sphere"`
	CastingTime  localized.Text `json:"casting_time"`
	Range        localized.Text `json:"range"`
	Duration     localized.Text `json:"duration"`
	AreaOfEffect localized.Text `json:"area_of_effect"`
	Components   localized.Text `json:"components"`
	SavingThrow  localized.Text `json:"saving_throw"`
	Description  localized.Text `json:"description"`
	// Source is the book code of the English text (PHB, TOM, ...; WEB when
	// no book was matched).
	Source string `json:"source"`

	// Marked are the gameplay values of the description and of the six stat texts
	// (ADR 0021 step 15): by field name, then language code, the text as spans that
	// concatenate to it, each value with its kind. Only a text with a value is
	// listed; the client shows any other plain.
	Marked map[string]map[string][]spellvalues.Span `json:"marked"`

	Stats       spellstats.Stats   `json:"stats"`
	CasterLevel int                `json:"caster_level,omitempty"`
	Values      *spellstats.Values `json:"values,omitempty"`
	// Damage is what a cast rolls at CasterLevel (absent: no caster level,
	// or a cast rolls no damage).
	Damage *spelleffectsvc.DamageAtWire `json:"damage,omitempty"`
	// Modes are the cast-time modes a cast or a receive picks from, each with
	// the damage a cast in it rolls at CasterLevel (the first is the default;
	// Damage is its damage). Absent: the spell has none.
	Modes []spelleffectsvc.ModeAtWire `json:"modes,omitempty"`
	// Exposure is the damage of one round of exposure to the spell's area
	// at CasterLevel (absent: none, or no caster level; ADR 0008 step 16).
	Exposure *spelleffectsvc.ExposureAtWire `json:"exposure,omitempty"`
	// Effect: POST /api/spells/effect takes the spell, it adds a buff
	// (absent: it adds none; ADR 0004 step 19).
	Effect bool `json:"effect,omitempty"`
}

// SlotsToDTO maps slots to their wire shape.
func SlotsToDTO(in []spellsvc.SpellSlot) []SpellSlotDTO {
	out := make([]SpellSlotDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellSlotDTO{Level: s.Level, PreparedSpellID: s.SpellID, PreparedSpellName: s.PreparedSpellName, Used: s.Used, Extra: s.Extra})
	}
	return out
}

// SpellToDTO maps one spell to its wire shape, with values at casterLevel.
func SpellToDTO(s spellsvc.Spell, casterLevel int) SpellDTO {
	dto := SpellDTO{
		ID: s.ID, Name: Text(s.Name), Class: s.Class, Level: s.Level,
		School: Text(s.School), Sphere: Text(s.Sphere),
		CastingTime: Text(s.CastingTime), Range: Text(s.Range), Duration: Text(s.Duration),
		AreaOfEffect: Text(s.AreaOfEffect), Components: Text(s.Components),
		SavingThrow: Text(s.SavingThrow), Description: Text(s.Description), Source: s.Source,
		Stats: s.Stats, CasterLevel: casterLevel,
		Marked: markValues(s),
	}
	if casterLevel > 0 {
		v := s.Stats.At(casterLevel)
		dto.Values = &v
	}
	return dto
}

// markValues is the one place where a spell's texts get their gameplay values marked
// (ADR 0021 step 15; at serve time, so moving it to the import only moves this call).
func markValues(s spellsvc.Spell) map[string]map[string][]spellvalues.Span {
	texts := map[string]localized.Text{
		"description": s.Description, "casting_time": s.CastingTime, "range": s.Range,
		"duration": s.Duration, "area_of_effect": s.AreaOfEffect,
		"components": s.Components, "saving_throw": s.SavingThrow,
	}
	out := map[string]map[string][]spellvalues.Span{}
	for _, field := range spellvalues.Fields {
		for lang, body := range texts[field] {
			if spans := spellvalues.Mark(lang, body); spellvalues.HasValue(spans) {
				if out[field] == nil {
					out[field] = map[string][]spellvalues.Span{}
				}
				out[field][lang] = spans
			}
		}
	}
	return out
}

package spellstats

import "strings"

// A label is the key of a common value of the book (ADR 0021 step 26.4): the parser sets it
// from the closed lists below, whole text only, and the web holds the wording of each key. A
// stat whose text is in no list has no label.

// Labels of the stat fields, by the field's name in Stats. They are the keys of the web's
// lookup.values.* (a test holds the two together).
var (
	castingTimeLabels = labelSet("special")
	rangeLabels       = labelSet("special", "touch", "unlimited")
	durationLabels    = labelSet("special", "permanent", "instantaneous")
	saveLabels        = labelSet("special", "none", "neg_", "neg", "1_2")
	areaLabels        = labelSet(
		"special", "hearing_range", "line_of_sight", "script_reader", "circle_of_priests",
		"variable", "varies", "the_caster_s_hand", "the_caster_s_shadow", "the_caster_s_staff",
		"the_wizard_s_staff", "caster_s_weapon", "the_casters", "the_caster", "caster",
		"one_creature", "1_creature", "creature_touched", "object_touched", "person_touched",
		"creature_s_touched", "creature_or_object_touched", "weapon_s_touched",
	)
)

func labelSet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// labelKey is the key of a text, the web's labelKey: trimmed, lowercased, every run of
// characters other than a letter or a digit is one "_" ("Neg." is "neg_", "Creature(s) touched"
// is "creature_s_touched").
func labelKey(raw string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			gap = false
			b.WriteRune(r)
		} else if !gap {
			gap = true
			b.WriteByte('_')
		}
	}
	return b.String()
}

// labelOf is the key of raw when the set lists it, else "".
func labelOf(set map[string]bool, raw string) string {
	if k := labelKey(raw); set[k] {
		return k
	}
	return ""
}

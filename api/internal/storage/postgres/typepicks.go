package postgres

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// typePicks is characters.proficiency_picks as the data steps read and write it before migration
// 042 turns a weapon pick into a catalogue weapon code (ADR 0037 step 21.8): the weapon picks as
// stored, a pick of the older form naming a weapon type ({"type", "slots"}), one of the newer a
// code ({"code", "slots"}); every other key (the non-weapon picks, the redo list) passes through
// as it is. The steps of migrations 026, 027 and 041 run on a database still in the older form,
// so they never read the column through the domain's picks.
type typePicks struct {
	other  map[string]json.RawMessage
	Weapon []typePick
}

// typePick is one stored weapon pick: Type for the older form, Code for the newer.
type typePick struct {
	Type  string `json:"type,omitempty"`
	Code  string `json:"code,omitempty"`
	Slots int    `json:"slots"`
}

// weaponKey is the column's key of the weapon picks.
const weaponKey = "weapon"

// decodeTypePicks reads the column; the other keys are kept for encode.
func decodeTypePicks(raw string) (typePicks, error) {
	out := typePicks{other: map[string]json.RawMessage{}}
	if err := json.Unmarshal([]byte(raw), &out.other); err != nil {
		return typePicks{}, fmt.Errorf("decode proficiency picks: %w", err)
	}
	if w, ok := out.other[weaponKey]; ok {
		if err := json.Unmarshal(w, &out.Weapon); err != nil {
			return typePicks{}, fmt.Errorf("decode weapon picks: %w", err)
		}
		delete(out.other, weaponKey)
	}
	return out, nil
}

// encode writes the column back: the other keys as read, the weapon picks when there are any.
func (p typePicks) encode() (string, error) {
	out := make(map[string]json.RawMessage, len(p.other)+1)
	for k, v := range p.other {
		out[k] = v
	}
	if len(p.Weapon) > 0 {
		w, err := json.Marshal(p.Weapon)
		if err != nil {
			return "", fmt.Errorf("encode weapon picks: %w", err)
		}
		out[weaponKey] = w
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode proficiency picks: %w", err)
	}
	return string(raw), nil
}

// hasType reports whether a weapon pick names weaponType.
func (p typePicks) hasType(weaponType string) bool {
	return slices.ContainsFunc(p.Weapon, func(w typePick) bool { return w.Type == weaponType })
}

// isTypeSpecializationOf is a type pick's specialization (ADR 0037 step 9 before step 21): a
// weapon type with the slots def's weapon_specialization params give it.
func (w typePick) isTypeSpecializationOf(def classes.Class) bool {
	return w.Slots > skills.WeaponPickSlots && slices.Contains(ruletablesvc.WeaponTypes, w.Type) &&
		w.Slots == ruletablesvc.SpecializationSlots(def, w.Type)
}

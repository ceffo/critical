package itemsvc

import (
	"context"
	"errors"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// Warning codes (ADR 0033 step 20): a class wears or wields what it should
// not. Language-neutral codes the web words (ADR 0023); a warning never
// refuses or changes anything.
const (
	// WarnArmourNotAllowed: a worn body armour or shield the class may not
	// wear.
	WarnArmourNotAllowed = "armour_not_allowed"
	// WarnWeaponNotAllowed: a wielded weapon outside the class's list.
	WarnWeaponNotAllowed = "weapon_not_allowed"
	// WarnHands: a two-handed weapon wielded with a worn shield (every
	// class), on both pieces.
	WarnHands = "hands"
	// WarnWeaponTypeMissing: a wielded weapon with no weapon type (every
	// class, ADR 0037 step 8): its attacks take no non-proficiency penalty,
	// since no pick can be judged, until the player sets its type; the app
	// never guesses one.
	WarnWeaponTypeMissing = "weapon_type_missing"
)

// Warning is one class warning, served with the piece's id.
type Warning struct {
	ItemID string
	Code   string
}

// ClassWarnings is the class warnings of the pieces worn or wielded, in list
// order (ADR 0033 step 20), and a wielded weapon's missing type (ADR 0037
// step 8): computed on every read, never stored. ruled says whether the
// class limits apply (a class the catalogue does not define has none); eq is
// the class's equipment (ADR 0040 step 1.6). A homebrew weapon without a
// catalogue code is judged by its weapon type and damage types; when those do
// not settle it (a type some listed weapon shares, such as other_melee for a
// thief), no warning.
func ClassWarnings(eq classes.Equipment, ruled bool, pieces []Item) []Warning {
	var out []Warning
	var shields, twoHanded []Item
	for _, p := range pieces {
		switch {
		case p.Category == CategoryShield && p.State == StateWorn:
			shields = append(shields, p)
			if ruled && !eq.Shields {
				out = append(out, Warning{p.ID, WarnArmourNotAllowed})
			}
		case p.Category == CategoryArmour && p.State == StateWorn:
			if ruled && !eq.AllowsArmour(p.SkillColumn) {
				out = append(out, Warning{p.ID, WarnArmourNotAllowed})
			}
		case p.Category == CategoryWeapon && p.State == StateWielded:
			if ruled && !eq.AllowsWeapon(wielded(p, pieces)) {
				out = append(out, Warning{p.ID, WarnWeaponNotAllowed})
			}
			if p.WeaponType == "" {
				out = append(out, Warning{p.ID, WarnWeaponTypeMissing})
			}
			if p.TwoHanded {
				twoHanded = append(twoHanded, p)
			}
		}
	}
	if len(shields) > 0 {
		for _, w := range twoHanded {
			out = append(out, Warning{w.ID, WarnHands})
		}
		if len(twoHanded) > 0 {
			for _, s := range shields {
				out = append(out, Warning{s.ID, WarnHands})
			}
		}
	}
	return out
}

// ammoDamageTypes are the damage types of an ammunition kind when no piece of that kind says
// (ADR 0033 step 4): bullets are bludgeoning, every other kind pierces.
var ammoDamageTypes = map[string][]string{"bullet": {"b"}}

// launcherDamageTypes are the damage types a launcher deals: its ammunition's (ADR 0033 step 4),
// from the pieces of the kind it fires, else the kind's own.
func launcherDamageTypes(p Item, pieces []Item) []string {
	var types []string
	for _, a := range pieces {
		if a.Category == CategoryAmmunition && a.AmmoKind == p.Fires {
			types = append(types, a.DamageTypes...)
		}
	}
	if len(types) > 0 {
		return types
	}
	if t, ok := ammoDamageTypes[p.Fires]; ok {
		return t
	}
	return []string{"p"}
}

// wielded is the piece as the class limits judge it: a launcher by its
// ammunition's damage types (PHB p.33: a sling is blunt by its bullets, a bow
// is not), any other weapon by its own.
func wielded(p Item, pieces []Item) classes.Weapon {
	types := p.DamageTypes
	if p.FiresAmmo() {
		types = launcherDamageTypes(p, pieces)
	}
	return classes.Weapon{CatalogueCode: p.CatalogueCode, WeaponType: p.WeaponType, DamageTypes: types}
}

// Warnings is the character's class warnings (ADR 0033 step 20), read from
// its class and its pieces: the list the caller already holds, or, with none,
// the character's pieces as stored.
func (s *service) Warnings(ctx context.Context, characterID int64, items []Item) ([]Warning, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		if items, err = s.dal.List(ctx, characterID); err != nil {
			return nil, err
		}
	}
	eq, ruled, err := s.equipmentOf(c.Class)
	if err != nil {
		return nil, err
	}
	return ClassWarnings(eq, ruled, items), nil
}

// equipmentOf is the class's equipment (ADR 0040 step 1.6); ruled is false
// when no class definitions are wired (no class limit then).
func (s *service) equipmentOf(class string) (eq classes.Equipment, ruled bool, err error) {
	if s.classes == nil {
		return classes.Equipment{}, false, nil
	}
	def, err := s.classes.Class(class)
	if errors.Is(err, classes.ErrUnknownClass) {
		return classes.Equipment{}, false, nil
	}
	if err != nil {
		return classes.Equipment{}, false, err
	}
	return def.Equipment, true, nil
}

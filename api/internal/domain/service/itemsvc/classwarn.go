package itemsvc

import (
	"context"
	"errors"
	"slices"

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
	// WarnSpellInArmour: a spell of a class that allows no body armour cast
	// while body armour is worn (ADR 0042 step 16.2, Q15), served on the
	// cast, never refusing it.
	WarnSpellInArmour = "spell_in_armour"
	// WarnOldAbility: a worn or wielded piece only a dual-class's old class
	// allows, before the crossover (ADR 0042 step 13,
	// classes.WarnOldAbility): allowed, warned.
	WarnOldAbility = classes.WarnOldAbility
)

// Warning is one class warning, served with the piece's id.
type Warning struct {
	ItemID string
	Code   string
}

// ClassWarnings is the class warnings of the pieces worn or wielded, in list
// order (ADR 0033 step 20), and a wielded weapon's missing type (ADR 0037
// step 8): computed on every read, never stored. limits are the equipment of
// the character's active class entries (ADR 0040 step 1.6), one for a
// single-class character; none (no entry, or one whose class the catalogue does
// not define, which has no limit) warns no class limit. Several are judged
// together (ADR 0042 step 16.1 and 16.2): a piece is allowed when any class
// allows it, except that a blunt_only class restricts every weapon to its own
// list. A homebrew weapon without a catalogue code is judged by its weapon type
// and damage types; when those do not settle it (a type some listed weapon
// shares, such as other_melee for a thief), no warning. old are a
// dual-class's old classes (ADR 0042 steps 13 and 16): a piece limits does not
// allow but one of them does is allowed, warned dual_class_old_ability while
// they are dormant.
func ClassWarnings(limits []classes.Equipment, old OldClasses, pieces []Item) []Warning {
	var out []Warning
	var shields, twoHanded []Item
	for _, p := range pieces {
		switch {
		case p.Category == CategoryShield && p.State == StateWorn:
			shields = append(shields, p)
			shield := func(e classes.Equipment) bool { return e.Shields }
			if code, ok := old.judged(anyClass(limits, shield), shield, WarnArmourNotAllowed); !ok {
				out = append(out, Warning{p.ID, code})
			}
		case p.Category == CategoryArmour && p.State == StateWorn:
			armour := func(e classes.Equipment) bool { return e.AllowsArmour(p.SkillColumn) }
			if code, ok := old.judged(anyClass(limits, armour), armour, WarnArmourNotAllowed); !ok {
				out = append(out, Warning{p.ID, code})
			}
		case p.Category == CategoryWeapon && p.State == StateWielded:
			w := wielded(p, pieces)
			weapon := func(e classes.Equipment) bool { return e.AllowsWeapon(w) }
			if code, ok := old.weapon(limits, weapon); !ok {
				out = append(out, Warning{p.ID, code})
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

// OldClasses are the equipment of a dual-class's frozen entries (ADR 0042
// steps 13 and 16), Dormant before the crossover. The zero value is none. An
// old class only adds what it allows: a blunt_only one restricts no weapon
// of the active classes, and it adds no weapon to an active blunt_only class
// (each action follows the limits of the class using it, step 13).
type OldClasses struct {
	Equipment []classes.Equipment
	Dormant   bool
}

// judged is a piece's verdict: ok when the active classes allow it (allowed)
// or an old class does past the crossover; dual_class_old_ability when an old
// class allows it before the crossover (ADR 0042 step 13); code otherwise.
func (old OldClasses) judged(allowed bool, allows func(classes.Equipment) bool, code string) (string, bool) {
	switch {
	case allowed:
		return "", true
	case !slices.ContainsFunc(old.Equipment, allows):
		return code, false
	case old.Dormant:
		return WarnOldAbility, false
	}
	return "", true
}

// weapon is a wielded weapon's verdict (ADR 0042 steps 13 and 16.1,
// classes.AllowedWithOld): allowed by the active classes, or by an old class
// (dual_class_old_ability before the crossover); an active blunt_only class
// decides alone, so an old class never lifts its limit, past the crossover too.
func (old OldClasses) weapon(limits []classes.Equipment, allows func(classes.Equipment) bool) (string, bool) {
	allowed, byOld := classes.AllowedWithOld(limits, old.Equipment, allows)
	switch {
	case !allowed:
		return WarnWeaponNotAllowed, false
	case byOld && old.Dormant:
		return WarnOldAbility, false
	}
	return "", true
}

// anyClass reports whether one of limits allows what allows judges; no limit
// at all is not judged (nothing warned).
func anyClass(limits []classes.Equipment, allows func(classes.Equipment) bool) bool {
	return len(limits) == 0 || slices.ContainsFunc(limits, allows)
}

// SpellInArmour is ADR 0042 step 16.2: a spell of a class that allows no body
// armour (classes.Equipment.NoArmour, which the caller judges) cast by a
// character of race wearing pieces warns spell_in_armour when body armour is
// worn, except an elf in elven chain (PHB multi/dual, Wizard). Whatever the
// character's other classes allow.
func SpellInArmour(race string, pieces []Item) bool {
	for _, p := range pieces {
		if p.Category != CategoryArmour || p.State != StateWorn || p.SkillColumn == classes.ArmourNone {
			continue
		}
		if race == "elf" && p.SkillColumn == "elven_chain" {
			continue
		}
		return true
	}
	return false
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
	entries := c.Entries()
	limits, err := s.limitsOf(c.ActiveClassIDs())
	if err != nil {
		return nil, err
	}
	// ADR 0042 steps 13 and 16: a dual-class's frozen classes, dormant
	// before the crossover. One the definitions do not know allows nothing
	// more.
	var frozen []string
	for _, e := range entries {
		if e.State == classes.EntryFrozen {
			frozen = append(frozen, e.Class)
		}
	}
	oldLimits, err := s.limitsOf(frozen)
	if err != nil {
		return nil, err
	}
	old := OldClasses{Equipment: oldLimits, Dormant: !classes.Crossed(entries)}
	return ClassWarnings(limits, old, items), nil
}

// limitsOf is the equipment of the active classes (ADR 0040 step 1.6, ADR
// 0042 step 16.1); nil when one of them is a class the definitions do not
// know, which has no class limit and so allows every piece.
func (s *service) limitsOf(active []string) ([]classes.Equipment, error) {
	limits := make([]classes.Equipment, 0, len(active))
	for _, class := range active {
		def, err := s.classes.Class(class)
		if errors.Is(err, classes.ErrUnknownClass) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		limits = append(limits, def.Equipment)
	}
	return limits, nil
}

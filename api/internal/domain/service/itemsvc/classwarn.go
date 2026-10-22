package itemsvc

import (
	"context"
	"slices"
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
)

// Warning is one class warning, served with the piece's id.
type Warning struct {
	ItemID string
	Code   string
}

// weaponRef is a PHB weapon the class lists, by catalogue code and the
// weapon type the catalogue gives it.
type weaponRef struct{ code, weaponType string }

// classRule is what a class may wear and wield (ADR 0033 step 20), data the
// checks read; a class with no rule (the fighter) is never warned.
type classRule struct {
	// noArmour: any worn body armour is warned; noShield: any worn shield.
	noArmour, noShield bool
	// armourColumns are the Table 29 columns of the body armour the class
	// may wear; empty: any.
	armourColumns []string
	// weaponTypes and weapons list the weapons the class may wield, by
	// weapon type or by catalogue code; both empty: any weapon.
	weaponTypes []string
	weapons     []weaponRef
	// bluntOnly: only a weapon with a bludgeoning (b) damage type.
	bluntOnly bool
}

// classRules are PHB p.39's lists (the mage's, the cleric's blunt weapons,
// the thief's and the bard's armour). The bard's weapons are unrestricted.
var classRules = map[string]classRule{
	"mage": {
		noArmour: true, noShield: true,
		weaponTypes: []string{"dagger", "dart", "sling"},
		weapons:     []weaponRef{{"knife", "other_melee"}, {"quarterstaff", "other_melee"}},
	},
	"cleric": {bluntOnly: true},
	"thief": {
		noShield:      true,
		armourColumns: []string{"none", "leather", "padded_studded", "elven_chain"},
		weaponTypes:   []string{"dagger", "dart", "hand_crossbow", "short_bow", "sling", "short_sword", "long_sword"},
		weapons: []weaponRef{
			{"club", "other_melee"},
			{"knife", "other_melee"},
			{"lasso", "other_thrown"},
			{"broad_sword", "other_melee"},
			{"quarterstaff", "other_melee"},
		},
	},
	"bard": {
		noShield:      true,
		armourColumns: []string{"none", "leather", "padded_studded", "elven_chain", "metal"},
	},
}

// ClassWarnings is the class warnings of the pieces worn or wielded, in list
// order (ADR 0033 step 20), and a wielded weapon's missing type (ADR 0037
// step 8): computed on every read, never stored. A homebrew
// weapon without a catalogue code is judged by its weapon type and damage
// types; when those do not settle it (a type some listed weapon shares, such
// as other_melee for a thief), no warning.
//
// classes are the character's active class entries (one for a single-class
// character). Several are judged together (ADR 0042 step 16.1 and 16.2): a
// piece is allowed when any class allows it, except that a blunt_only class
// restricts every weapon to its own list.
func ClassWarnings(classes []string, pieces []Item) []Warning {
	var out []Warning
	var shields, twoHanded []Item
	for _, p := range pieces {
		switch {
		case p.Category == CategoryShield && p.State == StateWorn:
			shields = append(shields, p)
			if !anyClass(classes, func(r classRule) bool { return !r.noShield }) {
				out = append(out, Warning{p.ID, WarnArmourNotAllowed})
			}
		case p.Category == CategoryArmour && p.State == StateWorn:
			if !anyClass(classes, func(r classRule) bool { return !r.noArmour && armourAllowed(r, p) }) {
				out = append(out, Warning{p.ID, WarnArmourNotAllowed})
			}
		case p.Category == CategoryWeapon && p.State == StateWielded:
			if !weaponAllowedTo(classes, p, pieces) {
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

// anyClass reports whether one of classes allows what allows judges; a class
// with no rule (the fighter) allows everything, and no class at all is not
// judged (nothing warned).
func anyClass(classes []string, allows func(classRule) bool) bool {
	if len(classes) == 0 {
		return true
	}
	for _, c := range classes {
		rule, ruled := classRules[c]
		if !ruled || allows(rule) {
			return true
		}
	}
	return false
}

// weaponAllowedTo is ADR 0042 step 16.1: a blunt_only class (the cleric)
// restricts the weapon to its own list whatever the other classes; otherwise
// the weapon is allowed when any class allows it.
func weaponAllowedTo(classes []string, p Item, pieces []Item) bool {
	for _, c := range classes {
		if rule, ruled := classRules[c]; ruled && rule.bluntOnly {
			return weaponAllowed(rule, p, pieces)
		}
	}
	return anyClass(classes, func(r classRule) bool { return weaponAllowed(r, p, pieces) })
}

// ForbidsArmour reports whether class allows no body armour (the mage): the
// classes whose spells ADR 0042 step 16.2 judges against the worn armour.
func ForbidsArmour(class string) bool {
	return classRules[class].noArmour
}

// SpellInArmour is ADR 0042 step 16.2: a spell of class cast by a character of
// race wearing pieces warns spell_in_armour when the class allows no body
// armour and body armour is worn, except an elf in elven chain (PHB
// multi/dual, Wizard). Whatever the character's other classes allow.
func SpellInArmour(class, race string, pieces []Item) bool {
	if !ForbidsArmour(class) {
		return false
	}
	for _, p := range pieces {
		if p.Category != CategoryArmour || p.State != StateWorn || p.SkillColumn == "none" {
			continue
		}
		if race == "elf" && p.SkillColumn == "elven_chain" {
			continue
		}
		return true
	}
	return false
}

func armourAllowed(r classRule, p Item) bool {
	return len(r.armourColumns) == 0 || slices.Contains(r.armourColumns, p.SkillColumn)
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

func weaponAllowed(r classRule, p Item, pieces []Item) bool {
	if r.bluntOnly {
		types := p.DamageTypes
		if p.FiresAmmo() {
			// PHB p.33 (only blunt, bludgeoning weapons): a launcher is judged by the
			// ammunition it fires, so a sling is allowed and a bow is not.
			types = launcherDamageTypes(p, pieces)
		}
		if len(types) > 0 {
			return slices.Contains(types, "b")
		}
		// No damage type: a catalogue row or a specific type (a bow) says
		// it does no bludgeoning; a bare homebrew weapon is not settled.
		return p.CatalogueCode == "" && (p.WeaponType == "" || p.WeaponType == "other_melee" ||
			p.WeaponType == "other_thrown" || p.WeaponType == "other_missile")
	}
	if len(r.weaponTypes) == 0 && len(r.weapons) == 0 {
		return true
	}
	if slices.Contains(r.weaponTypes, p.WeaponType) {
		return true
	}
	if p.CatalogueCode != "" {
		return slices.ContainsFunc(r.weapons, func(w weaponRef) bool { return w.code == p.CatalogueCode })
	}
	// Homebrew: unsettled when the type is one a listed weapon has.
	return p.WeaponType == "" || slices.ContainsFunc(r.weapons, func(w weaponRef) bool { return w.weaponType == p.WeaponType })
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
	return ClassWarnings(c.ActiveClassIDs(), items), nil
}

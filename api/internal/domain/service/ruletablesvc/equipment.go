package ruletablesvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// The equipment reference data of catalogue format 7 (ADR 0021 step 13, ADR
// 0033 steps 21 to 23): the weapons, ammunition, body armour and shields a
// player picks from, and the two rule tables PHB Table 52 and Table 56. Numbers
// and codes; the names are language maps (ADR 0023). A character's piece copies
// a row and keeps its code, so nothing here is referenced by game state (step
// 23).

// ArmorRows are the PHB Table 52 armour types (ADR 0033 step 5), the codes an
// armour row's ArmorRow and a weapon_vs_armor row use.
var ArmorRows = []string{
	"banded", "brigandine", "chain", "field_plate", "full_plate", "leather", "plate", "ring", "scale", "splint",
	"studded_leather",
}

// MagicKinds are the magic item kinds (ADR 0033 step 2), the rows of the
// initiative_item table.
var MagicKinds = []string{"potion", "scroll", "ring", "rod", "staff", "wand", "misc"}

// ScrollKind is the magic kind whose initiative modifier is the spell's
// casting time: its row carries no modifier.
const ScrollKind = "scroll"

// WeaponVsArmorRow is a PHB Table 52 row: the modifier of one armour type
// against each damage type (ADR 0033 steps 19 and 21).
type WeaponVsArmorRow struct {
	ArmorRow                string
	Slash, Pierce, Bludgeon int
}

// InitiativeItemRow is a PHB Table 56 row: the initiative modifier of a magic
// item kind. A scroll has none (HasModifier false): the spell's casting time
// counts.
type InitiativeItemRow struct {
	Kind        string
	Modifier    int
	HasModifier bool
}

// WeaponVsArmor is the validated Table 52: exactly one row per armour row.
type WeaponVsArmor struct {
	rows map[string]WeaponVsArmorRow
}

// NewWeaponVsArmor validates the rows (the rule the schema cannot express,
// ADR 0021 step 13.3): every armour row exactly once. A violation names it.
func NewWeaponVsArmor(rows []WeaponVsArmorRow) (*WeaponVsArmor, error) {
	if err := once(rows, func(r WeaponVsArmorRow) string { return r.ArmorRow }, ArmorRows); err != nil {
		return nil, err
	}
	out := &WeaponVsArmor{rows: make(map[string]WeaponVsArmorRow, len(rows))}
	for _, r := range rows {
		out.rows[r.ArmorRow] = r
	}
	return out, nil
}

// Row returns the Table 52 row of an armour row code.
func (w *WeaponVsArmor) Row(armorRow string) (WeaponVsArmorRow, bool) {
	r, ok := w.rows[armorRow]
	return r, ok
}

// InitiativeItems is the validated Table 56: one row per magic kind.
type InitiativeItems struct {
	rows map[string]InitiativeItemRow
}

// NewInitiativeItems validates the rows: every magic kind exactly once, a
// modifier on every kind but the scroll, none on the scroll.
func NewInitiativeItems(rows []InitiativeItemRow) (*InitiativeItems, error) {
	if err := once(rows, func(r InitiativeItemRow) string { return r.Kind }, MagicKinds); err != nil {
		return nil, err
	}
	out := &InitiativeItems{rows: make(map[string]InitiativeItemRow, len(rows))}
	for _, r := range rows {
		switch {
		case r.Kind == ScrollKind && r.HasModifier:
			return nil, fmt.Errorf("%q: a scroll has no modifier (the spell's casting time counts)", r.Kind)
		case r.Kind != ScrollKind && !r.HasModifier:
			return nil, fmt.Errorf("%q: no modifier", r.Kind)
		}
		out.rows[r.Kind] = r
	}
	return out, nil
}

// Row returns the Table 56 row of a magic kind.
func (i *InitiativeItems) Row(kind string) (InitiativeItemRow, bool) {
	r, ok := i.rows[kind]
	return r, ok
}

// Weapon is one catalogue weapon (PHB Table 44). A launcher (Fires not empty)
// has a speed but no damage: its ammunition carries the dice (ADR 0033 step
// 4). Damage is as printed ("1d6+1").
type Weapon struct {
	Code       string
	Name       localized.Text
	WeaponType string
	Kind       string
	Size       string
	Types      []string
	Speed      int
	DamageSM   string
	DamageL    string
	TwoHanded  bool
	Fires      []string
}

// Ammunition is one catalogue ammunition row.
type Ammunition struct {
	Code     string
	Name     localized.Text
	AmmoKind string
	Types    []string
	DamageSM string
	DamageL  string
}

// Armour is one catalogue body armour (ADR 0033 step 5).
type Armour struct {
	Code        string
	Name        localized.Text
	BaseAC      int
	SkillColumn string
	ArmorRow    string
}

// Shield is one catalogue shield.
type Shield struct {
	Code               string
	Name               localized.Text
	Kind               string
	ACMelee, ACMissile int
}

// Equipment is the validated equipment reference data, in the artifact's
// order.
type Equipment struct {
	Weapons    []Weapon
	Ammunition []Ammunition
	Armour     []Armour
	Shields    []Shield
}

// NewEquipment validates the rules the schema cannot express (ADR 0021 step
// 13.3): a code names one row across the four lists; a launcher is a missile
// weapon with no damage of its own and fires ammunition kinds some row
// carries; every other weapon has damage unless it has no damage type
// (mancatcher), and its damage comes in pairs. A violation names the row.
func NewEquipment(e Equipment) (*Equipment, error) {
	seen := map[string]bool{}
	use := func(code string) error {
		if seen[code] {
			return fmt.Errorf("code %q twice", code)
		}
		seen[code] = true
		return nil
	}
	kinds := map[string]bool{}
	for _, a := range e.Ammunition {
		if err := use(a.Code); err != nil {
			return nil, fmt.Errorf("ammunition: %w", err)
		}
		kinds[a.AmmoKind] = true
	}
	for _, w := range e.Weapons {
		if err := use(w.Code); err != nil {
			return nil, fmt.Errorf("weapons: %w", err)
		}
		if len(w.Fires) > 0 && (w.Kind != "missile" || w.DamageSM != "" || w.DamageL != "") {
			return nil, fmt.Errorf("weapon %q: a launcher is a missile weapon with no damage of its own", w.Code)
		}
		for _, k := range w.Fires {
			if !kinds[k] {
				return nil, fmt.Errorf("weapon %q: fires %q, which no ammunition row carries", w.Code, k)
			}
		}
		if len(w.Fires) == 0 && len(w.Types) > 0 && w.DamageSM == "" {
			return nil, fmt.Errorf("weapon %q: no damage and no ammunition kind", w.Code)
		}
	}
	for _, a := range e.Armour {
		if err := use(a.Code); err != nil {
			return nil, fmt.Errorf("armour: %w", err)
		}
		if !slices.Contains(ArmorRows, a.ArmorRow) {
			return nil, fmt.Errorf("armour %q: unknown armour row %q", a.Code, a.ArmorRow)
		}
	}
	for _, s := range e.Shields {
		if err := use(s.Code); err != nil {
			return nil, fmt.Errorf("shields: %w", err)
		}
	}
	return &e, nil
}

// WithEquipment returns a copy of t carrying the format-7 equipment data.
func (t *Tables) WithEquipment(e *Equipment, w *WeaponVsArmor, i *InitiativeItems) *Tables {
	out := *t
	out.equipment, out.weaponVsArmor, out.initiativeItems = e, w, i
	return &out
}

// Equipment returns the equipment reference data, false before format 7.
func (t *Tables) Equipment() (*Equipment, bool) { return t.equipment, t.equipment != nil }

// WeaponVsArmor returns PHB Table 52, false before format 7.
func (t *Tables) WeaponVsArmor() (*WeaponVsArmor, bool) {
	return t.weaponVsArmor, t.weaponVsArmor != nil
}

// InitiativeItems returns PHB Table 56's magic item rows, false before format 7.
func (t *Tables) InitiativeItems() (*InitiativeItems, bool) {
	return t.initiativeItems, t.initiativeItems != nil
}

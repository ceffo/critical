package itemsvc

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// LegacyWeapon is a weapon as the weapons table stored it before the one
// equipment model (ADR 0033 step 26): the input of the conversion, and what a
// "weapons:<id>" snapshot holds.
type LegacyWeapon struct {
	ID          string
	Position    int
	Name        localized.Text
	DamageSM    string
	DamageL     string
	MagicToHit  int
	MagicDamage int
	Kind        string
	TwoHanded   bool
	AmmoKind    string // the ammunition it fires
	Type        string // weapon type code
	Specialized bool
}

// Conversion is one character's equipment converted (ADR 0033 step 26): the
// pieces in list order, the review rows (step 27) and the items whose id a
// weapon already had, by old id (step 26.3).
type Conversion struct {
	Pieces  []Item
	Reviews []Review
	Renames map[string]string
}

// matchable are the weapon types a catalogue row is matched by (ADR 0033 step
// 26.4): each the code of the row of that type; other_melee, other_thrown,
// other_missile and none are not.
var matchable = []string{
	TypeLongSword, TypeShortSword, TypeShortBow, TypeLongBow, TypeDagger, TypeDart, TypeSling,
	TypeLightCrossbow, TypeHeavyCrossbow, TypeHandCrossbow,
}

// Convert is the conversion of one character's items and weapons (ADR 0033
// step 26), a pure function of them and of the catalogue's weapons (none
// before catalogue format 7: no weapon is then matched):
//
//  1. every item and weapon keeps its id, its fields and its name; the
//     items keep their positions and the weapons follow them, in their order;
//  2. (the snapshots are the restorers', see Restorers);
//  3. an item whose id a weapon has takes the next free suffix, recorded in
//     Renames;
//  4. a weapon is matched to the catalogue row of its type, else to the one
//     row its name (without a magic suffix such as "+2" or "+1/+2") or id
//     names unambiguously (matchByName), which adds the code, the size, the
//     damage types and the speed and overwrites no stored stat; an unmatched
//     or ambiguous one is on the review list;
//  5. an item is categorised from its type label when it is obvious (an item
//     with an ammo kind is ammunition); ammunition with no dice gets the dice
//     of the first launcher that fires its kind; the rest is gear on the
//     review list;
//  6. every weapon is wielded, every piece with powers worn (one that cannot
//     be worn stays carried, on the review list), the rest carried.
func Convert(items []Item, weapons []LegacyWeapon, catalogue []ruletablesvc.Weapon) Conversion {
	return ConvertWithHistory(items, weapons, catalogue, History{})
}

// History is the ids the action log still holds snapshots of, for items and
// weapons that are no longer stored: undoing their removal (or redoing their
// add) brings them back, so their identities count as taken (ADR 0033 step
// 26.3).
type History struct {
	ItemIDs   []string // ids of "items:<id>" snapshots
	WeaponIDs []string // ids of "weapons:<id>" snapshots
}

// ConvertWithHistory is Convert that also keeps apart an item and a weapon
// that share an id only through the log: an item (stored or only in the log)
// whose id a weapon (stored or only in the log) has takes the next free
// suffix, recorded in Renames, so that restoring either never overwrites the
// other. A renamed id with no stored item is recorded only: the restorer of an
// old items snapshot reads it.
func ConvertWithHistory(items []Item, weapons []LegacyWeapon, catalogue []ruletablesvc.Weapon, h History) Conversion {
	out := Conversion{Renames: map[string]string{}}
	taken := map[string]bool{}
	weaponIDs := map[string]bool{}
	storedItems := map[string]bool{}
	end := 0
	for _, w := range weapons {
		taken[w.ID], weaponIDs[w.ID] = true, true
	}
	for _, id := range h.WeaponIDs {
		taken[id], weaponIDs[id] = true, true
	}
	for _, it := range items {
		taken[it.ID], storedItems[it.ID] = true, true
		if it.Position >= end {
			end = it.Position + 1
		}
	}
	for _, id := range h.ItemIDs {
		taken[id] = true
	}
	nextFree := func(base string) string {
		for n := 2; ; n++ {
			id := base + "_" + strconv.Itoa(n)
			if !taken[id] {
				taken[id] = true
				return id
			}
		}
	}
	for _, it := range items {
		piece, reasons := ConvertItem(it)
		if weaponIDs[it.ID] {
			id := nextFree(it.ID)
			out.Renames[it.ID] = id
			piece.ID = id
		}
		out.Pieces = append(out.Pieces, piece)
		out.Reviews = append(out.Reviews, reviews(piece.ID, reasons)...)
	}
	for _, id := range slices.Sorted(slices.Values(h.ItemIDs)) {
		if _, done := out.Renames[id]; !storedItems[id] && weaponIDs[id] && !done {
			out.Renames[id] = nextFree(id)
		}
	}
	ordered := slices.Clone(weapons)
	slices.SortStableFunc(ordered, func(a, b LegacyWeapon) int { return a.Position - b.Position })
	for i, w := range ordered {
		piece, reasons := ConvertWeapon(w, catalogue)
		piece.Position = end + i
		out.Pieces = append(out.Pieces, piece)
		out.Reviews = append(out.Reviews, reviews(piece.ID, reasons)...)
	}
	launcherDice(out.Pieces)
	return out
}

func reviews(id string, reasons []string) []Review {
	out := make([]Review, 0, len(reasons))
	for _, r := range reasons {
		out = append(out, Review{ItemID: id, Reason: r})
	}
	return out
}

// launcherDice gives ammunition with no dice those of the first launcher, in
// list order, that fires its kind (ADR 0033 step 26.5), so the attack rolls
// the same.
func launcherDice(pieces []Item) {
	for i := range pieces {
		a := &pieces[i]
		if a.Category != CategoryAmmunition || a.DamageSM != "" || a.DamageL != "" {
			continue
		}
		for _, w := range pieces {
			if w.FiresAmmo() && w.Fires == a.AmmoKind && w.DamageSM != "" {
				a.DamageSM, a.DamageL = w.DamageSM, w.DamageL
				break
			}
		}
	}
}

// labels are the item type labels the conversion reads (ADR 0033 step
// 26.5), English singular and plural: an ammunition kind, a magic kind or
// gear.
var labels = map[string]struct {
	category Category
	value    string // the ammo kind or the magic kind
}{
	"arrow": {CategoryAmmunition, "arrow"}, "quarrel": {CategoryAmmunition, "quarrel"},
	"bolt": {CategoryAmmunition, "quarrel"}, "bullet": {CategoryAmmunition, "bullet"},
	"potion": {CategoryMagic, MagicPotion}, "scroll": {CategoryMagic, MagicScroll},
	"ring": {CategoryMagic, MagicRing}, "rod": {CategoryMagic, MagicRod}, "staff": {CategoryMagic, MagicStaff},
	"staves": {CategoryMagic, MagicStaff}, "wand": {CategoryMagic, MagicWand}, "gem": {CategoryMagic, MagicMisc},
	"flask": {CategoryGear, ""},
}

func label(t string) (Category, string, bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	if l, ok := labels[t]; ok {
		return l.category, l.value, true
	}
	if l, ok := labels[strings.TrimSuffix(t, "s")]; ok && strings.HasSuffix(t, "s") {
		return l.category, l.value, true
	}
	return "", "", false
}

// component is the component code a type label names (ADR 0033 step 1.1:
// the label a spell's consume_item matched, case-insensitive, singular or
// plural), empty for none.
func component(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	for _, c := range Components {
		if t == c || t == c+"s" {
			return c
		}
	}
	return ""
}

// ammoKinds reads the ammo kinds typed before the model (ADR 0033 step 4's
// codes, their plurals, and bolt for quarrel); any other is kept as typed.
func ammoKind(kind string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(kind))
	if k == "" {
		return "", true
	}
	if c, v, ok := label(k); ok && c == CategoryAmmunition {
		return v, true
	}
	k = strings.ReplaceAll(k, " ", "_")
	if slices.Contains(AmmoKinds, k) {
		return k, true
	}
	if t := strings.TrimSuffix(k, "s"); slices.Contains(AmmoKinds, t) {
		return t, true
	}
	return strings.TrimSpace(kind), false
}

// ConvertItem is one item of before the model as a piece (ADR 0033 step
// 26.5 and 26.6), with its review reasons. Every stored field is kept.
func ConvertItem(it Item) (Item, []string) {
	var reasons []string
	piece := it
	piece.State = StateCarried
	piece.Component = component(it.Type)
	cat, value, known := label(it.Type)
	switch {
	case strings.TrimSpace(it.AmmoKind) != "" || (known && cat == CategoryAmmunition):
		piece.Category = CategoryAmmunition
		kind := it.AmmoKind
		if strings.TrimSpace(kind) == "" {
			kind = value
		}
		k, ok := ammoKind(kind)
		piece.AmmoKind = k
		if !ok {
			reasons = append(reasons, ReasonAmmoKindUnknown)
		}
	case known:
		piece.Category = cat
		if cat == CategoryMagic {
			piece.MagicKind = value
		}
	default:
		piece.Category = CategoryGear
		reasons = append(reasons, ReasonUncategorised)
	}
	if len(it.Powers) > 0 {
		// The powers stay on (step 9): an uncategorised piece with powers is
		// a magic one.
		if piece.Category == CategoryGear {
			piece.Category, piece.MagicKind = CategoryMagic, MagicMisc
			if !slices.Contains(reasons, ReasonUncategorised) {
				reasons = append(reasons, ReasonUncategorised)
			}
		}
		if Allows(piece.Category, piece.MagicKind, StateWorn) {
			piece.State = StateWorn
		} else {
			reasons = append(reasons, ReasonPowersOff)
		}
	}
	return piece, reasons
}

// ConvertWeapon is one weapon of before the model as a wielded piece (ADR 0033
// steps 26.4 and 26.6), matched to the catalogue row of its type, with its
// review reasons. Every stored stat is kept; its count is one, in stock.
func ConvertWeapon(w LegacyWeapon, catalogue []ruletablesvc.Weapon) (Item, []string) {
	var reasons []string
	fires, ok := ammoKind(w.AmmoKind)
	if !ok {
		reasons = append(reasons, ReasonAmmoKindUnknown)
	}
	piece := Item{
		ID: w.ID, Position: w.Position, Name: w.Name, Category: CategoryWeapon, State: StateWielded,
		Behaviour: BehaviourStock, Count: 1, MaxCount: 1,
		DamageSM: w.DamageSM, DamageL: w.DamageL, MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage,
		WeaponKind: w.Kind, TwoHanded: w.TwoHanded, Fires: fires, WeaponType: w.Type, Specialized: w.Specialized,
	}
	row, matched := match(w.Type, catalogue)
	if !matched {
		row, matched = matchByName(w, catalogue)
	}
	if !matched {
		return piece, append(reasons, ReasonWeaponUnmatched)
	}
	piece.CatalogueCode, piece.Size, piece.Speed = row.Code, row.Size, row.Speed
	piece.DamageTypes = slices.Clone(row.Types)
	return piece, reasons
}

// match is the catalogue row of a matchable weapon type: the row whose code
// is the type and whose weapon type is it too.
func match(weaponType string, catalogue []ruletablesvc.Weapon) (ruletablesvc.Weapon, bool) {
	if !slices.Contains(matchable, weaponType) {
		return ruletablesvc.Weapon{}, false
	}
	for _, row := range catalogue {
		if row.Code == weaponType && row.WeaponType == weaponType {
			return row, true
		}
	}
	return ruletablesvc.Weapon{}, false
}

// magicSuffix is the magic bonus a weapon's name may end with: "+2", "+1/+2".
var magicSuffix = regexp.MustCompile(`\s*\(?\+\d+(\s*/\s*\+\d+)?\)?\s*$`)

// matchByName is the catalogue row a weapon that no type matched names
// unambiguously (ADR 0033 step 26.4): of the rows of its kind, those whose
// code is its id or its name (any language, case ignored, the magic suffix
// dropped) written with underscores, or whose name is its name; failing that
// those whose name is its name followed by a qualifier after a comma
// ("Bastard sword, two-handed"), kept only when the weapon's two-handed flag
// is the row's. Exactly one row matches, or none: nothing is guessed.
func matchByName(w LegacyWeapon, catalogue []ruletablesvc.Weapon) (ruletablesvc.Weapon, bool) {
	var bases []string
	for _, n := range w.Name {
		if b := strings.ToLower(strings.TrimSpace(magicSuffix.ReplaceAllString(n, ""))); b != "" {
			bases = append(bases, b)
		}
	}
	id := strings.ToLower(w.ID)
	exact := func(row ruletablesvc.Weapon) bool {
		if row.Code == id {
			return true
		}
		for _, b := range bases {
			if row.Code == strings.ReplaceAll(b, " ", "_") {
				return true
			}
		}
		for _, n := range row.Name {
			if slices.Contains(bases, strings.ToLower(strings.TrimSpace(n))) {
				return true
			}
		}
		return false
	}
	qualified := func(row ruletablesvc.Weapon) bool {
		if row.TwoHanded != w.TwoHanded {
			return false
		}
		for _, n := range row.Name {
			for _, b := range bases {
				if strings.HasPrefix(strings.ToLower(n), b+", ") || strings.HasPrefix(strings.ToLower(n), b+" (") {
					return true
				}
			}
		}
		return false
	}
	for _, test := range []func(ruletablesvc.Weapon) bool{exact, qualified} {
		var found []ruletablesvc.Weapon
		for _, row := range catalogue {
			if row.Kind == w.Kind && test(row) {
				found = append(found, row)
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		if len(found) > 1 {
			return ruletablesvc.Weapon{}, false
		}
	}
	return ruletablesvc.Weapon{}, false
}

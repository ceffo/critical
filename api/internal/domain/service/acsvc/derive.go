package acsvc

import (
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// UnarmouredAC is the base AC with no body armour and no base_ac effect
// (PHB Table 46: "none", 10).
const UnarmouredAC = 10

// Inputs is everything the AC reads, as the services serve it.
type Inputs struct {
	// Items are the character's pieces in list order (worn armour and
	// shield, the catalogue codes of the pieces whose powers protect).
	Items []itemsvc.Item
	// Buffs are the active buffs with whether each applies (ADR 0004 steps
	// 10 and 11).
	Buffs []buffsvc.BuffStatus
	// Powers are the powers that are on: those of the worn or wielded
	// pieces whose trigger is met (ADR 0033 step 9, ADR 0016 step 10), in
	// item then power order.
	Powers []buffsvc.ItemPower
	// Dexterity is the sheet's score and DexAdjust its PHB Table 2
	// defensive adjustment (the catalogue's dexterity.defensive).
	Dexterity int
	DexAdjust int
}

// source is where an AC effect comes from: a buff or a piece's power.
type source struct {
	itemID, buff, spellID, proficiencyID string
	effects                              buffsvc.BuffEffects
}

func (s source) row(t Term, v int) Row {
	return Row{Term: t, ItemID: s.itemID, Buff: s.buff, SpellID: s.spellID, ProficiencyID: s.proficiencyID, Value: v}
}

// forAC reports whether an effect set may move the character's AC: one
// conditioned on the weapon kind of the character's own attack (ADR 0004
// step 18) counts only for that attack, never for the AC.
func forAC(e buffsvc.BuffEffects) bool { return e.WeaponKind == buffsvc.WeaponNone }

// Derive is the character's AC (ADR 0033 step 10), its terms in this order,
// each one or more rows of the breakdown with its source:
//
//  1. Base: the worn body armour's base AC; with none worn, the best
//     (lowest) base_ac effect that applies (step 11), else 10.
//  2. Armour magic: the worn armour's ac powers.
//  3. Shield: minus the worn shield's AC bonus (its missile bonus for the
//     AC against missiles), then its ac powers.
//  4. Dexterity: PHB Table 2's defensive adjustment for the sheet's score.
//  5. Protection: the ac powers of the other equipped pieces, in list
//     order; a without_magic_armour effect counts only while the worn armour
//     has no ac power, and of two pieces of one catalogue code only the best
//     counts (step 12).
//  6. Buffs: the ac effects of the buffs that apply, in list order (with the
//     same without_magic_armour condition).
//  7. Floor: never better than -10 (DMG p.181).
//
// A row that does not add (step 12's two cases, a base_ac that is not the
// base) is shown with the reason. Derive is pure: the same inputs give the
// same AC, nothing is stored (step 10: derived, never entered).
func Derive(in Inputs) AC {
	armour := wornPiece(in.Items, itemsvc.CategoryArmour)
	shield := wornPiece(in.Items, itemsvc.CategoryShield)
	pieces := make(map[string]itemsvc.Item, len(in.Items))
	for _, it := range in.Items {
		pieces[it.ID] = it
	}

	var buffs, powers []source
	for _, b := range in.Buffs {
		if b.Applies && forAC(b.Effects) {
			buffs = append(buffs, source{buff: b.Name, spellID: b.SpellID, proficiencyID: b.ProficiencyID, effects: b.Effects})
		}
	}
	for _, p := range in.Powers {
		if forAC(p.Effects) {
			powers = append(powers, source{itemID: p.ItemID, effects: p.Effects})
		}
	}
	of := func(id string) []source {
		out := []source{}
		for _, p := range powers {
			if id != "" && p.itemID == id {
				out = append(out, p)
			}
		}
		return out
	}
	armourID, shieldID := "", ""
	if armour != nil {
		armourID = armour.ID
	}
	if shield != nil {
		shieldID = shield.ID
	}
	// Step 12: the worn body armour is magic when one of its powers moves
	// the AC.
	magicArmour := false
	for _, p := range of(armourID) {
		if acOf(p.effects) != 0 {
			magicArmour = true
		}
	}
	offFor := func(e buffsvc.BuffEffects) Off {
		if e.WithoutMagicArmour && magicArmour {
			return OffMagicArmour
		}
		return ""
	}

	rows := baseRows(armour, append(append([]source{}, buffs...), powers...))

	// Step 10.2.
	armourRows := []Row{}
	for _, p := range of(armourID) {
		if v := acOf(p.effects); v != 0 {
			armourRows = append(armourRows, p.row(TermArmourMagic, v))
		}
	}
	if len(armourRows) == 0 {
		armourRows = append(armourRows, Row{Term: TermArmourMagic})
	}
	rows = append(rows, armourRows...)

	// Step 10.3.
	if shield != nil {
		r := Row{Term: TermShield, ItemID: shield.ID, Value: -shield.ACMelee}
		if shield.ACMissile != shield.ACMelee {
			m := -shield.ACMissile
			r.MissileValue = &m
		}
		rows = append(rows, r)
		for _, p := range of(shieldID) {
			if v := acOf(p.effects); v != 0 {
				rows = append(rows, p.row(TermShieldMagic, v))
			}
		}
	} else {
		rows = append(rows, Row{Term: TermShield})
	}

	// Step 10.4.
	rows = append(rows, Row{Term: TermDexterity, Value: in.DexAdjust, Dexterity: in.Dexterity})

	// Step 10.5.
	rows = append(rows, protectionRows(powers, pieces, armourID, shieldID, offFor)...)

	// Step 10.6.
	for _, b := range buffs {
		if v := acOf(b.effects); v != 0 {
			r := b.row(TermBuffs, v)
			r.Off = offFor(b.effects)
			rows = append(rows, r)
		}
	}

	return total(rows)
}

// acOf is an effect set's ac change, 0 for none.
func acOf(e buffsvc.BuffEffects) int {
	if e.AC == nil {
		return 0
	}
	return *e.AC
}

// wornPiece is the worn piece of category c (at most one, ADR 0033 step 6),
// or nil.
func wornPiece(items []itemsvc.Item, c itemsvc.Category) *itemsvc.Item {
	for i := range items {
		if items[i].Category == c && items[i].State == itemsvc.StateWorn {
			return &items[i]
		}
	}
	return nil
}

// baseRows is step 10.1: the counting base row first, then the base_ac
// effects that do not count (a body armour is worn, or a better one won;
// the first wins a tie).
func baseRows(armour *itemsvc.Item, sources []source) []Row {
	var cands []source
	for _, s := range sources {
		if s.effects.BaseAC != nil {
			cands = append(cands, s)
		}
	}
	if armour != nil {
		rows := []Row{{Term: TermBase, ItemID: armour.ID, Value: armour.BaseAC}}
		for _, c := range cands {
			r := c.row(TermBase, *c.effects.BaseAC)
			r.Off = OffArmourWorn
			rows = append(rows, r)
		}
		return rows
	}
	if len(cands) == 0 {
		return []Row{{Term: TermBase, Value: UnarmouredAC}}
	}
	best := 0
	for i, c := range cands {
		if *c.effects.BaseAC < *cands[best].effects.BaseAC {
			best = i
		}
	}
	rows := []Row{cands[best].row(TermBase, *cands[best].effects.BaseAC)}
	for i, c := range cands {
		if i != best {
			r := c.row(TermBase, *c.effects.BaseAC)
			r.Off = OffNotBest
			rows = append(rows, r)
		}
	}
	return rows
}

// protectionRows is step 10.5 with step 12: the ac powers of the equipped
// pieces other than the worn armour and shield, in list order. Of the pieces
// that share a catalogue code, only the one whose powers better the AC most
// counts (the first on a tie); the others' rows are shown as duplicates.
func protectionRows(powers []source, pieces map[string]itemsvc.Item, armourID, shieldID string, offFor func(buffsvc.BuffEffects) Off) []Row {
	rows := []Row{}
	sums := map[string]int{} // a piece's adding ac total
	var order []string       // pieces in the order their first row comes
	for _, p := range powers {
		if p.itemID == armourID || p.itemID == shieldID {
			continue
		}
		v := acOf(p.effects)
		if v == 0 {
			continue
		}
		r := p.row(TermProtection, v)
		r.Off = offFor(p.effects)
		rows = append(rows, r)
		if _, seen := sums[p.itemID]; !seen {
			order = append(order, p.itemID)
			sums[p.itemID] = 0
		}
		if r.Off == "" {
			sums[p.itemID] += v
		}
	}
	// The best piece of each catalogue code.
	best := map[string]string{}
	for _, id := range order {
		code := pieces[id].CatalogueCode
		if code == "" || sums[id] == 0 {
			continue
		}
		if cur, ok := best[code]; !ok || sums[id] < sums[cur] {
			best[code] = id
		}
	}
	for i := range rows {
		code := pieces[rows[i].ItemID].CatalogueCode
		if b, ok := best[code]; ok && code != "" && b != rows[i].ItemID && rows[i].Off == "" {
			rows[i].Off = OffDuplicate
		}
	}
	return rows
}

// total sums the rows that add (step 10) and floors the sum at -10 (step
// 10.7), against melee and, with a body shield, against missiles.
func total(rows []Row) AC {
	melee, missile := 0, 0
	differ := false
	for _, r := range rows {
		if r.Off != "" {
			continue
		}
		melee += r.Value
		if r.MissileValue != nil {
			missile += *r.MissileValue
			differ = true
		} else {
			missile += r.Value
		}
	}
	out := AC{Rows: rows, Value: max(melee, buffsvc.MinAC), Floored: melee < buffsvc.MinAC}
	if m := max(missile, buffsvc.MinAC); differ && m != out.Value {
		out.Missile = &m
	}
	return out
}

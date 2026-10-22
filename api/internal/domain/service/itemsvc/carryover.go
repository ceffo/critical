package itemsvc

import (
	"errors"
	"fmt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ReasonArmourGuessed is the review reason (ADR 0033 step 27) of a body
// armour the skill armour carry-over guessed (step 14): an entered "padded or
// studded leather" became Studded leather, a bard's "metal armour up to chain
// mail" Chain mail; the player says which armour it is.
const ReasonArmourGuessed = "armour_guessed"

// The entered skill armours (ADR 0030 step 1, as stored in
// characters.skill_armor), the Table 29 column each is, and the catalogue row
// the carry-over gives for each (ADR 0033 step 14). "" (none entered) reads
// as leather; "none" gives nothing.
var carriedArmour = map[string]struct {
	column, code string
	guessed      bool
}{
	"":                       {column: ruletablesvc.ColumnLeather, code: "leather"},
	"leather":                {column: ruletablesvc.ColumnLeather, code: "leather"},
	"padded_studded_leather": {column: ruletablesvc.ColumnPaddedStudded, code: "studded_leather", guessed: true},
	"elven_chain":            {column: ruletablesvc.ColumnElvenChain, code: "elven_chain_mail"},
	"metal":                  {column: ruletablesvc.ColumnMetal, code: "chain_mail", guessed: true},
}

// ErrCarryOverArmourMissing is a carry-over that needs a catalogue armour
// row the catalogue does not hold (a catalogue before format 7).
var ErrCarryOverArmourMissing = errors.New("the catalogue has no armour row for the skill armour carry-over")

// CarryOver is what the skill armour carry-over does to one character's
// equipment: nothing (the zero value), wear a piece it carries (Wear, its
// id), or add a new worn piece (Add) with, when the piece is a guess, its
// review row (Review).
type CarryOver struct {
	Wear   string
	Add    *Item
	Review *Review
}

// WornSkillColumn is the Table 29 column of the body armour among pieces that
// is worn (at most one, ADR 0033 step 6), "" when none is: what a thief's or
// a bard's skills read (step 14).
func WornSkillColumn(pieces []Item) string {
	for _, p := range pieces {
		if p.Category == CategoryArmour && p.State == StateWorn {
			return p.SkillColumn
		}
	}
	return ""
}

// CarrySkillArmour is ADR 0033 step 14's carry-over for one character of
// class def, which entered skillArmor (characters.skill_armor) and holds pieces,
// in list order. armours are the catalogue's body armours. A character
// whose class lists no rogue_skills (ADR 0040 step 3), one that wears a body armour already, and an entered
// "none" get nothing. Otherwise an armour of the entered Table 29 column (""
// reads as leather) is worn: the first carried piece of that column, else a new piece from the catalogue row the
// entry names (carriedArmour: its name in every language the catalogue has,
// its stats, worn, at the end of the list, one count). So every skill
// percentage is the same before and after. A guessed new piece is on the
// review list (ReasonArmourGuessed). ErrCarryOverArmourMissing when the row
// is not in armours; an entered value that is none of the armours is an
// error (the column's CHECK refuses one).
func CarrySkillArmour(def classes.Class, skillArmor string, pieces []Item, armours []ruletablesvc.Armour) (CarryOver, error) {
	if _, ok := def.Lists(classes.KindRogueSkills); !ok || skillArmor == "none" {
		return CarryOver{}, nil
	}
	for _, p := range pieces {
		if p.Category == CategoryArmour && p.State == StateWorn {
			return CarryOver{}, nil
		}
	}
	target, ok := carriedArmour[skillArmor]
	if !ok {
		return CarryOver{}, fmt.Errorf("skill armour carry-over: unknown skill armour %q", skillArmor)
	}
	for _, p := range pieces {
		if p.Category == CategoryArmour && p.SkillColumn == target.column {
			return CarryOver{Wear: p.ID}, nil
		}
	}
	var row *ruletablesvc.Armour
	for i := range armours {
		if armours[i].Code == target.code {
			row = &armours[i]
			break
		}
	}
	if row == nil {
		return CarryOver{}, fmt.Errorf("%w: %q", ErrCarryOverArmourMissing, target.code)
	}
	taken := map[string]bool{}
	position := 0
	for _, p := range pieces {
		taken[p.ID] = true
		position = max(position, p.Position+1)
	}
	id := row.Code
	for n := 2; taken[id]; n++ {
		id = row.Code + "_" + strconv.Itoa(n)
	}
	piece := Item{
		ID: id, Position: position, Name: row.Name.Trimmed(), Behaviour: BehaviourStock, Count: 1, MaxCount: 1,
		Category: CategoryArmour, CatalogueCode: row.Code, State: StateWorn,
		BaseAC: row.BaseAC, SkillColumn: row.SkillColumn, ArmorRow: row.ArmorRow,
	}
	out := CarryOver{Add: &piece}
	if target.guessed {
		out.Review = &Review{ItemID: id, Reason: ReasonArmourGuessed}
	}
	return out, nil
}

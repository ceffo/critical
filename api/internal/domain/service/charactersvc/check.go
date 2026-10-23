package charactersvc

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// checked is a new character's entries read field by field (check): the character they enter,
// the refusals in the form's order (Create answers the first), the fields still blank (a draft's;
// Create refuses them), whether the class and XP read (levelled: the starting level and the skill
// point budget need nothing else) and whether every entry the derivation reads does (derivable:
// the class, race, scores, skill points and XP). The picks and the languages are judged later, at
// the starting level (skillsAt).
type checked struct {
	c         Character
	refusals  []error
	missing   []string
	levelled  bool
	derivable bool
	// defs are the class and race definitions the check read.
	defs *classes.Registry
	// def is the entered class's definition (zero for a class missing or
	// unknown).
	def classes.Class
	// strOK: the Strength is entered and in range.
	strOK bool
	// ents are the class entries read (the one-class case's too); active are the definitions of the
	// active ones, in order.
	ents   []NewClass
	active []classes.Class
	// exceptional: an active entry's class lists exceptional_strength, the Strength entered is 18
	// and the race rolls it.
	exceptional bool
	// alignments are the alignments every active entry's class admits (alignmentsOfEntries).
	alignments []string
}

// alignmentsOfEntries are the alignments every active entry's class admits (ADR 0042 step 2: each
// class's alignments apply), in the first class's order; every one of the nine before a class is
// chosen.
func alignmentsOfEntries(defs *classes.Registry, entries []classes.Entry) []string {
	var out []string
	first := true
	for _, e := range entries {
		if e.State != classes.EntryActive {
			continue
		}
		a := AlignmentsOf(defs, e.Class)
		if first {
			out, first = slices.Clone(a), false
			continue
		}
		out = slices.DeleteFunc(out, func(x string) bool { return !slices.Contains(a, x) })
	}
	if first {
		return slices.Clone(classes.Alignments)
	}
	return out
}

// check reads n field by field, in the form's order. The creation (draft false) stops at the
// first refusal. A draft lists a blank name, class, race, alignment or sex and the numbers named
// in blank as missing instead of refusing them, and goes on past a refusal so that every field's
// verdict is read; every refusal it gives is the creation's, field and code alike.
func (s *service) check(ctx context.Context, n NewCharacter, blank []string, draft bool) (checked, error) {
	var out checked
	defs, err := s.rules.Definitions()
	if err != nil {
		return checked{}, fmt.Errorf("new character: %w", err)
	}
	out.defs = defs
	stop := func() bool { return !draft && len(out.refusals) > 0 }
	refuse := func(err error) { out.refusals = append(out.refusals, err) }
	empty := func(field string) {
		if draft {
			out.missing = append(out.missing, field)
		} else {
			refuse(&domain.FieldError{Field: field, Message: CodeEmpty})
		}
	}
	name, err := cleanName(n.Name)
	switch {
	case err == nil:
	case strings.TrimSpace(n.Name) == "":
		empty("name")
	default:
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	ents := n.entries()
	out.ents = ents
	classOK := true
	var skillDef classes.Class
	skillFound := false
	for i, e := range ents {
		class, ok := creatableClass(defs, e.Class)
		switch {
		case ok:
			if e.frozen() {
				continue
			}
			out.active = append(out.active, class)
			// Skill points go to the first entry whose class lists rogue skills (ADR 0030 steps 17 and 18).
			if _, rogue := class.Lists(classes.KindRogueSkills); rogue && !skillFound {
				skillDef, skillFound = class, true
			}
		case draft && e.Class == "":
			empty(n.field(i, "class"))
			classOK = false
		default:
			refuse(refusal(n.field(i, "class"), CodeNotCreatable))
			classOK = false
		}
	}
	if classOK {
		// ADR 0042 step 1: the entries as a whole.
		for _, err := range classShape(n) {
			refuse(err)
			classOK = false
		}
		// ADR 0042 step 2.1: a class the catalogue marks as never part of a multi-class.
		for _, err := range multiClassClassRefusals(n, defs) {
			refuse(err)
			classOK = false
		}
	}
	switch {
	case skillFound:
		out.def = skillDef
	case len(out.active) > 0:
		out.def = out.active[0]
	}
	if stop() {
		return out, nil
	}
	raceOK := false
	switch {
	case n.Race == "":
		empty("race")
	case !raceKnown(defs, n.Race):
		refuse(refusal("race", CodeUnknown))
	case classOK:
		rs, err := s.ruleset(ctx)
		if err != nil {
			return checked{}, fmt.Errorf("new character: %w", err)
		}
		refusals, err := raceRefusals(rs.RacialLift, defs, n, s.rules.RaceAllows)
		if err != nil {
			return checked{}, fmt.Errorf("new character: %w", err)
		}
		for _, r := range refusals {
			refuse(r)
		}
		raceOK = len(refusals) == 0
	}
	if stop() {
		return out, nil
	}
	scoresOK := true
	for _, a := range []struct {
		field string
		v     int
	}{
		{string(AbilityStr), n.Str},
		{string(AbilityDex), n.Dex},
		{string(AbilityCon), n.Con},
		{string(AbilityInt), n.Int},
		{string(AbilityWis), n.Wis},
		{string(AbilityCha), n.Cha},
	} {
		switch {
		case slices.Contains(blank, a.field):
			out.missing, scoresOK = append(out.missing, a.field), false
		case abilityRefusal(a.field, a.v) != nil:
			refuse(abilityRefusal(a.field, a.v))
			scoresOK = false
		case a.field == string(AbilityStr):
			out.strOK = true
		}
		if stop() {
			return out, nil
		}
	}
	if scoresOK && classOK {
		for _, err := range primeRefusals(n, defs, map[string]int{
			"str": n.Str, "dex": n.Dex, "con": n.Con, "int": n.Int, "wis": n.Wis, "cha": n.Cha,
		}) {
			refuse(err)
		}
		if stop() {
			return out, nil
		}
	}
	race := raceOf(defs, n.Race)
	if err := validExceptional(n, out.active, race); err != nil {
		refuse(err)
	}
	for _, def := range out.active {
		out.exceptional = out.exceptional || (out.strOK && ruletablesvc.ExceptionalStrengthAllowed(def, race, n.Str))
	}
	if stop() {
		return out, nil
	}
	points, err := validSkills(n, out.def)
	pointsOK := err == nil
	if err != nil {
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	switch {
	case slices.Contains(blank, "hpMax"):
		out.missing = append(out.missing, "hpMax")
	case hpMaxRefusal(n.HPMax) != nil:
		refuse(hpMaxRefusal(n.HPMax))
	}
	if err := pspMaxRefusal(n.PSPMax); err != nil {
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	xpOK := true
	entries := make([]classes.Entry, 0, len(ents))
	for i, e := range ents {
		f := n.field(i, "xp")
		switch {
		case e.frozen():
		case slices.Contains(blank, f):
			out.missing, xpOK = append(out.missing, f), false
		}
		if err := xpRefusal(f, e.XP); err != nil {
			refuse(err)
			xpOK = false
		}
		entry := classes.Entry{Class: e.Class, RealizedLevel: e.Level, StartingXP: e.XP, State: classes.EntryActive}
		if e.frozen() {
			entry.State, entry.RealizedLevel, entry.LeftAtLevel = classes.EntryFrozen, e.LeftAtLevel, e.LeftAtLevel
			if e.XP == 0 && classOK && e.LeftAtLevel >= 1 && e.LeftAtLevel <= classes.MaxLevel {
				// A class left keeps the XP that reaches its left-at level when none is entered.
				if entry.StartingXP, err = s.rules.XPThreshold(e.Class, e.LeftAtLevel); err != nil {
					return checked{}, fmt.Errorf("new character: %w", err)
				}
			}
		}
		entries = append(entries, entry)
	}
	out.alignments = alignmentsOfEntries(defs, entries)
	details, missing, err := newDetails(out.alignments, n.Details, !draft)
	if err != nil {
		refuse(err)
	}
	out.missing = append(out.missing, missing...)
	out.levelled = classOK && xpOK
	out.derivable = out.levelled && raceOK && scoresOK && pointsOK
	out.c = Character{
		Name: name, Class: ents[0].Class, Race: n.Race,
		Str: n.Str, StrExceptional: n.StrExceptional, Dex: n.Dex, Con: n.Con, Int: n.Int, Wis: n.Wis, Cha: n.Cha,
		HPMax: n.HPMax, XPTotal: n.XP, Details: details, SkillPoints: points,
	}
	if len(n.Classes) > 0 {
		out.c.SetClasses(entries)
	}
	return out, nil
}

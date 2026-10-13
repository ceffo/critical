package spellsvc

import (
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// Ref names a catalog spell for key assignment.
type Ref struct {
	Name  string
	Class string
	Level int
}

// Keys returns the unique key of every spell of the catalog, in refs order.
// This is the one place the rule lives:
//
//   - a Wizard spell is Slug(name) ("magic_missile"), so every id in
//     spellbook.json, spell_effects.json and the fr catalogue stays valid;
//   - a spell with a same-named, lower-level spell in its class takes
//     the suffix "_l<level>" ("fortify_l4");
//   - a Priest spell whose name a Wizard spell also has takes "_priest" after
//     the slug ("hold_person_priest", or "..._priest_l4" for a higher level),
//     so it never resolves to the Wizard's effects.
func Keys(refs []Ref) []string {
	lowest := map[string]int{}  // class|slug -> lowest level
	wizard := map[string]bool{} // slugs a Wizard spell owns
	for _, r := range refs {
		s := spelleffectsvc.Slug(r.Name)
		k := strings.ToLower(r.Class) + "|" + s
		if l, ok := lowest[k]; !ok || r.Level < l {
			lowest[k] = r.Level
		}
		if strings.EqualFold(r.Class, "wizard") {
			wizard[s] = true
		}
	}
	out := make([]string, len(refs))
	for i, r := range refs {
		s := spelleffectsvc.Slug(r.Name)
		priestOfWizardName := !strings.EqualFold(r.Class, "wizard") && wizard[s]
		base := s
		if priestOfWizardName {
			base = s + "_priest"
		}
		if r.Level != lowest[strings.ToLower(r.Class)+"|"+s] {
			base += "_l" + strconv.Itoa(r.Level)
		}
		out[i] = base
	}
	return out
}

package charactersvc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0043 step 3: per effect tag the best bonus, the first entry's on a tie, never the sum of two
// classes' bonuses. The fixture has one class granting save_bonus, so the grants are built here.
func TestClassSaveBonuses_Step3_TheBestPerTagNeverSummed(t *testing.T) {
	grant := func(class, against string, bonus int) classes.Grant {
		return classes.Grant{
			Class:   classes.Class{ID: class},
			Feature: classes.Feature{SaveBonus: &classes.SaveBonus{Against: against, Bonus: bonus}},
		}
	}
	es := []classes.Entry{entry("a", 5), entry("b", 5)}
	for _, tc := range []struct {
		name string
		gs   []classes.Grant
		want []ClassSaveBonus
	}{
		{
			"a tie keeps the first entry's, not the sum",
			[]classes.Grant{grant("a", "mind", 2), grant("b", "mind", 2)},
			[]ClassSaveBonus{{Class: "a", Against: "mind", Bonus: 2}},
		},
		{
			"the better bonus wins, whichever entry has it",
			[]classes.Grant{grant("a", "mind", 2), grant("b", "mind", 3)},
			[]ClassSaveBonus{{Class: "b", Against: "mind", Bonus: 3}},
		},
		{
			"the first's stays when it is the better",
			[]classes.Grant{grant("a", "mind", 3), grant("b", "mind", 2)},
			[]ClassSaveBonus{{Class: "a", Against: "mind", Bonus: 3}},
		},
		{
			"two tags each keep their own",
			[]classes.Grant{grant("a", "mind", 2), grant("b", "poison", 1)},
			[]ClassSaveBonus{{Class: "a", Against: "mind", Bonus: 2}, {Class: "b", Against: "poison", Bonus: 1}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classSaveBonuses(es, tc.gs))
		})
	}
}

func entry(class string, level int) classes.Entry {
	return classes.Entry{Class: class, RealizedLevel: level, State: classes.EntryActive}
}

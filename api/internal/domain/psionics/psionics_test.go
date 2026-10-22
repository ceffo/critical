package psionics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// book is a small catalogue: two disciplines, sciences s1..s3 and devotions
// d1..d8 in telepathy, one science and four devotions in clairsentience, a
// defence mode and a devotion that needs the science s1.
func book() psionics.Book {
	disciplines := []catalogue.Discipline{{ID: "telepathy"}, {ID: "clairsentience"}}
	var powers []catalogue.Power
	add := func(id, discipline, kind string, pre ...string) {
		powers = append(powers, catalogue.Power{ID: id, Discipline: discipline, Kind: kind, Prerequisites: pre})
	}
	for _, id := range []string{"s1", "s2", "s3"} {
		add(id, "telepathy", catalogue.KindScience)
	}
	for _, id := range []string{"d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8"} {
		add(id, "telepathy", catalogue.KindDevotion)
	}
	add("c_s1", "clairsentience", catalogue.KindScience)
	for _, id := range []string{"c_d1", "c_d2", "c_d3", "c_d4"} {
		add(id, "clairsentience", catalogue.KindDevotion)
	}
	add("needs_s1", "telepathy", catalogue.KindDevotion, "s1")
	add("mode", "telepathy", catalogue.KindDefenceMode)
	return psionics.NewBook(disciplines, powers)
}

func disc(ids ...string) []psionics.Pick { return picks(psionics.PickDiscipline, ids...) }
func pow(ids ...string) []psionics.Pick  { return picks(psionics.PickPower, ids...) }

func picks(kind string, ids ...string) []psionics.Pick {
	out := make([]psionics.Pick, 0, len(ids))
	for _, id := range ids {
		out = append(out, psionics.Pick{Kind: kind, ID: id})
	}
	return out
}

func join(lists ...[]psionics.Pick) []psionics.Pick {
	var out []psionics.Pick
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

func refusal(t *testing.T, err error) *psionics.Refusal {
	t.Helper()
	var r *psionics.Refusal
	require.ErrorAs(t, err, &r)
	require.ErrorIs(t, err, psionics.ErrRefused)
	return r
}

// Level 1 (Table 4): 1 discipline, 1 science, 3 devotions, 1 defence mode.
func TestPicks_Step9_TableFourAllowance(t *testing.T) {
	a := psionics.Allowance{Disciplines: 1, Sciences: 1, Devotions: 3, DefenceModes: 1}
	full := join(disc("telepathy"), pow("s1", "d1", "d2", "d3", "mode"))
	used, err := psionics.Validate(book(), a, full)
	require.NoError(t, err)
	assert.Equal(t, psionics.Used{Disciplines: 1, Sciences: 1, Devotions: 3, DefenceModes: 1}, used)

	r := refusal(t, func() error { _, e := psionics.Validate(book(), a, join(full, pow("d4"))); return e }())
	assert.Equal(t, psionics.CodeAllowanceExceeded, r.Code)
	assert.Equal(t, psionics.SlotDevotion, r.Slot)
	assert.Equal(t, 4, r.Count)
	assert.Equal(t, 3, r.Limit)

	_, err = psionics.Validate(book(), a, join(disc("telepathy", "clairsentience")))
	assert.Equal(t, psionics.SlotDiscipline, refusal(t, err).Slot)
	_, err = psionics.Validate(book(), a, join(disc("telepathy"), pow("s1", "s2")))
	assert.Equal(t, psionics.SlotScience, refusal(t, err).Slot)
}

// The book's example: 3 primary sciences allow 2 in another discipline, never
// 3; a discipline's devotions are at least twice its sciences.
func TestPicks_Step9_PrimaryAndRatio(t *testing.T) {
	a := psionics.Allowance{Disciplines: 2, Sciences: 9, Devotions: 30, DefenceModes: 1}
	primary := join(disc("telepathy", "clairsentience"), pow("s1", "s2", "s3", "d1", "d2", "d3", "d4", "d5", "d6"))
	_, err := psionics.Validate(book(), a, primary)
	require.NoError(t, err)

	// Another discipline's sciences stay under the primary's: 1 of 3 is fine.
	_, err = psionics.Validate(book(), a, join(primary, pow("c_s1", "c_d1", "c_d2")))
	require.NoError(t, err)

	// A primary with one science and two devotions: the other's one science
	// would equal it.
	tight := join(disc("telepathy", "clairsentience"), pow("s1", "d1", "d2", "c_s1", "c_d1", "c_d2"))
	_, err = psionics.Validate(book(), a, tight)
	r := refusal(t, err)
	assert.Equal(t, psionics.CodePrimaryExceeded, r.Code)
	assert.Equal(t, "clairsentience", r.Discipline)
	assert.Equal(t, psionics.SlotScience, r.Slot)

	// The other discipline's devotions must stay under the primary's too.
	_, err = psionics.Validate(book(), a, join(disc("telepathy", "clairsentience"), pow("d1", "d2", "c_d1", "c_d2")))
	assert.Equal(t, psionics.CodePrimaryExceeded, refusal(t, err).Code)

	// Two sciences with three devotions: devotions under twice the sciences.
	_, err = psionics.Validate(book(), a, join(disc("telepathy"), pow("s1", "s2", "d1", "d2", "d3")))
	r = refusal(t, err)
	assert.Equal(t, psionics.CodeDevotionsBelowSciences, r.Code)
	assert.Equal(t, 3, r.Count)
	assert.Equal(t, 4, r.Limit)
}

// A power's discipline must be picked (a defence mode excepted) and its
// prerequisites known.
func TestPicks_Step9_PrerequisitesAndRelearn(t *testing.T) {
	a := psionics.Allowance{Disciplines: 2, Sciences: 3, Devotions: 8, DefenceModes: 1}
	_, err := psionics.Validate(book(), a, join(disc("clairsentience"), pow("d1")))
	r := refusal(t, err)
	assert.Equal(t, psionics.CodeDisciplineNotKnown, r.Code)
	assert.Equal(t, "d1", r.Power)

	_, err = psionics.Validate(book(), a, join(disc("telepathy"), pow("needs_s1")))
	r = refusal(t, err)
	assert.Equal(t, psionics.CodePrerequisiteNotKnown, r.Code)
	assert.Equal(t, "needs_s1", r.Power)

	// A defence mode needs no discipline.
	_, err = psionics.Validate(book(), a, pow("mode"))
	require.NoError(t, err)

	// A relearn takes the place of a devotion and is of a power known.
	list := join(disc("telepathy"), pow("d1"), picks(psionics.PickRelearn, "d1"))
	used, err := psionics.Validate(book(), a, list)
	require.NoError(t, err)
	assert.Equal(t, 2, used.Devotions)

	for _, bad := range [][]psionics.Pick{
		join(disc("telepathy"), picks(psionics.PickRelearn, "d2")), // not known
		join(disc("telepathy"), pow("mode"), picks(psionics.PickRelearn, "mode")),
		join(disc("telepathy", "telepathy")),
		pow("nope"),
		{{Kind: "x", ID: "y"}},
	} {
		_, err = psionics.Validate(book(), a, bad)
		assert.ErrorIs(t, err, psionics.ErrInvalid)
	}
}

// A relearn takes a slot of the allowance (9.1) and nothing more: it is not
// another science or devotion of its discipline for steps 9.2 and 9.3.
func TestPicks_Step9_RelearnCountsInAllowanceOnly(t *testing.T) {
	a := psionics.Allowance{Disciplines: 2, Sciences: 4, Devotions: 8, DefenceModes: 1}
	both := disc("telepathy", "clairsentience")

	// The primary's one science, relearned, is still one: the other
	// discipline's one science equals it and is refused.
	list := join(both, pow("s1", "d1", "d2", "c_s1", "c_d1", "c_d2"), picks(psionics.PickRelearn, "s1"))
	_, err := psionics.Validate(book(), a, list)
	r := refusal(t, err)
	assert.Equal(t, psionics.CodePrimaryExceeded, r.Code)
	assert.Equal(t, "clairsentience", r.Discipline)
	assert.Equal(t, psionics.SlotScience, r.Slot)

	// A science relearned with exactly two devotions in its discipline stays
	// accepted, and takes a science slot.
	used, err := psionics.Validate(book(), a, join(disc("telepathy"), pow("s1", "d1", "d2"), picks(psionics.PickRelearn, "s1")))
	require.NoError(t, err)
	assert.Equal(t, 2, used.Sciences)
}

// A primary with nothing to compare: the refusal's limit is 0, never -1.
func TestPicks_Step9_PrimaryLimitNotNegative(t *testing.T) {
	a := psionics.Allowance{Disciplines: 2, Sciences: 3, Devotions: 8, DefenceModes: 1}
	_, err := psionics.Validate(book(), a, join(disc("telepathy", "clairsentience"), pow("c_s1", "c_d1", "c_d2")))
	r := refusal(t, err)
	assert.Equal(t, psionics.CodePrimaryExceeded, r.Code)
	assert.Equal(t, 0, r.Limit)
}

// ADR 0043 step 9.1 (with ADR 0021 step 23): an attack mode takes a devotion's
// slot unless the catalogue names another for it; the one that names a science
// takes a science's slot, read from the data and not from its id.
func TestPicks_Step9_AnAttackModeTakesTheSlotTheCatalogueNames(t *testing.T) {
	powers := []catalogue.Power{
		{ID: "plain_attack", Discipline: "telepathy", Kind: catalogue.KindAttackMode},
		{ID: "science_attack", Discipline: "telepathy", Kind: catalogue.KindAttackMode, Slot: catalogue.KindScience},
		{ID: "d1", Discipline: "telepathy", Kind: catalogue.KindDevotion},
		{ID: "d2", Discipline: "telepathy", Kind: catalogue.KindDevotion},
	}
	b := psionics.NewBook([]catalogue.Discipline{{ID: "telepathy"}}, powers)
	assert.Equal(t, psionics.SlotDevotion, psionics.SlotOf(powers[0]))
	assert.Equal(t, psionics.SlotScience, psionics.SlotOf(powers[1]))

	a := psionics.Allowance{Disciplines: 1, Sciences: 1, Devotions: 2}
	used, err := psionics.Validate(b, a, join(disc("telepathy"), pow("science_attack", "d1", "d2")))
	require.NoError(t, err)
	assert.Equal(t, psionics.Used{Disciplines: 1, Sciences: 1, Devotions: 2}, used, "the science-slot attack mode takes a science, not a devotion")

	used, err = psionics.Validate(b, psionics.Allowance{Disciplines: 1, Devotions: 1}, join(disc("telepathy"), pow("plain_attack")))
	require.NoError(t, err)
	assert.Equal(t, psionics.Used{Disciplines: 1, Devotions: 1}, used)

	// With one science, the second pick of that slot is refused.
	_, err = psionics.Validate(b, a, join(disc("telepathy"), pow("science_attack", "plain_attack", "d1", "d2")))
	assert.Equal(t, psionics.CodeAllowanceExceeded, refusal(t, err).Code)
}

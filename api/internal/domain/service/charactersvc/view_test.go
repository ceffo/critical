package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0045: the character view is a projection of one derivation. The party's
// fixtures: a single-class character built from the mirror alone, a
// multi-class one and the dual-class elf on both sides of the crossover.
func viewFixtures() map[string]*charactersvc.Character {
	mirror := &charactersvc.Character{
		ID: 5, Name: "Solo", Race: "human", Class: "fighter", RealizedLevel: 5, XPTotal: 16_000,
		Str: 17, StrExceptional: 0, Dex: 14, Con: 15, Int: 10, Wis: 12, Cha: 9, HPMax: 40,
	}
	return map[string]*charactersvc.Character{
		"single mirror":        mirror,
		"multi":                multi("half_elf", entry("fighter", 5, 16_000), entry("mage", 4, 5_000)),
		"dual before crossing": elf(7, 60_000),
		"dual after crossing":  elf(10, 300_000),
	}
}

// ADR 0045 step 6.3: the view built from Derive carries the entity's
// derived block and picks field by field; a figure forgotten in buildView
// fails here.
func TestView_Step6_ProjectsTheDerivedBlock(t *testing.T) {
	for name, c := range viewFixtures() {
		t.Run(name, func(t *testing.T) {
			c.Derived = derived(t, c)
			v := charactersvc.BuildView(*c, c.Derived)
			d := c.Derived

			assert.Equal(t, c.ID, v.ID)
			assert.Equal(t, c.Name, v.Name)
			assert.Equal(t, c.Race, v.Race)
			assert.Equal(t, []int{c.Str, c.StrExceptional, c.Dex, c.Con, c.Int, c.Wis, c.Cha, c.HPMax, c.PSPMax},
				[]int{v.Str, v.StrExceptional, v.Dex, v.Con, v.Int, v.Wis, v.Cha, v.HPMax, v.PSPMax})
			assert.Equal(t, d.THAC0, v.THAC0)
			assert.Equal(t, d.Saves, v.Saves)
			assert.Equal(t, d.RaceSaveBonus, v.RaceSaveBonus)
			assert.Equal(t, d.MagicDefence, v.MagicDefence)
			assert.Equal(t, d.ClassSaveBonuses, v.ClassSaveBonuses)
			assert.Equal(t, d.SpellImmunities, v.SpellImmunities)
			assert.Equal(t, d.SpellSlots, v.SpellSlots)
			assert.Equal(t, d.Specializes, v.Specializes)
			assert.Equal(t, d.Spellbook, v.Spellbook)
			assert.Equal(t, d.Turning, v.Turning)
			assert.Equal(t, d.SkillColumn, v.SkillColumn)
			assert.Equal(t, d.SkillSets, v.SkillSets)
			assert.Equal(t, d.Backstab, v.Backstab)
			assert.Equal(t, d.ExceptionalStrength, v.ExceptionalStrength)
			assert.Equal(t, d.SpellFailure, v.SpellFailure)
			assert.Equal(t, d.Proficiencies, v.Proficiencies)
			assert.Equal(t, d.PSP, v.PSP)
			assert.Equal(t, d.Powers, v.Powers)
			assert.Equal(t, d.Features, v.Features)
			assert.Equal(t, c.SkillPoints, v.SkillPoints)
			assert.Equal(t, c.Proficiencies, v.ProficiencyPicks)
			assert.Equal(t, c.PsionicPicks, v.PsionicPicks)

			stored := c.Entries()
			require.Len(t, v.Entries, len(d.Classes))
			for i, e := range v.Entries {
				ed := d.Classes[i]
				assert.Equal(t, charactersvc.EntryView{
					Class: ed.Class, State: ed.State, StartingXP: stored[i].StartingXP, LeftAtLevel: stored[i].LeftAtLevel,
					Group: ed.Group, XP: ed.XP, TargetLevel: ed.TargetLevel, RealizedLevel: ed.RealizedLevel,
					NextLevelXP: ed.NextLevelXP, SpellSlots: ed.SpellSlots, Casts: ed.Casts, NoArmour: ed.NoArmour,
					SpellFailure: ed.SpellFailure, LevelUp: ed.LevelUp, Dormant: ed.Dormant,
				}, e)
			}

			// The readers moved onto the view answer as the derived block did.
			assert.Equal(t, d.CasterLevel(), v.CasterLevel())
			assert.Equal(t, d.Casters(), v.Casters())
			assert.Equal(t, d.Groups(), v.Groups())
			assert.Equal(t, d.HasDormant(), v.HasDormant())
			assert.Equal(t, d.FeatureLevel(classes.KindSpellbook), v.FeatureLevel(classes.KindSpellbook))
			assert.Equal(t, d.SkillGrants("thief"), v.SkillGrants("thief"))
			for _, kind := range []string{classes.KindSpellbook, classes.KindRogueSkills} {
				dg, dok := d.Grant(kind)
				vg, vok := v.Grant(kind)
				assert.Equal(t, dg, vg)
				assert.Equal(t, dok, vok)
			}
		})
	}
}

// ADR 0045 step 3.4 (Q3): the one SkillClasses gives what StoredClasses (the
// stored row) and ProficiencyClasses (the derived block) gave, on the party's
// fixtures; a case where they differ fails here before either is removed.
func TestView_Step3_SkillClassesEqualsStoredAndProficiencyClasses(t *testing.T) {
	for name, c := range viewFixtures() {
		t.Run(name, func(t *testing.T) {
			c.Derived = derived(t, c)
			v := charactersvc.BuildView(*c, c.Derived)

			assert.Equal(t, c.StoredClasses(), v.SkillClasses())
			assert.Equal(t, c.ProficiencyClasses(), v.SkillClasses())
			assert.Equal(t, c.ActiveClasses(), activeClasses(v))
		})
	}
	// The dual-class's old class is kept at its left-at level, frozen.
	c := elf(7, 60_000)
	c.Derived = derived(t, c)
	assert.Equal(t, skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "mage", Level: 7}},
		charactersvc.BuildView(*c, c.Derived).SkillClasses())
}

func activeClasses(v charactersvc.View) skills.Classes {
	var out skills.Classes
	for _, e := range v.Active() {
		out = append(out, skills.ClassLevel{Class: e.Class, Level: e.RealizedLevel})
	}
	return out
}

// ADR 0045 step 3.1: Primary is the first active entry, whatever the order
// of the entries; the table of cases is the web helper's too (step 5.2).
func TestView_Step3_PrimaryIsTheFirstActive(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []viewtest.Option
		want    string
	}{
		{"a single active entry", []viewtest.Option{viewtest.WithEntry("mage", 3)}, "mage"},
		{"several active entries", []viewtest.Option{viewtest.WithEntry("fighter", 5), viewtest.WithEntry("mage", 4)}, "fighter"},
		{"a frozen entry first", []viewtest.Option{viewtest.WithFrozen("fighter", 9), viewtest.WithEntry("mage", 10)}, "mage"},
		{"no active entry", []viewtest.Option{viewtest.WithFrozen("fighter", 9)}, "fighter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, viewtest.Multi(tc.options...).Primary().Class)
		})
	}

	// Through the derivation, the primary entry is the one the derived
	// figures of the character are (Derived.RealizedLevel).
	c := elf(10, 300_000)
	c.Derived = derived(t, c)
	p := charactersvc.BuildView(*c, c.Derived).Primary()
	assert.Equal(t, "mage", p.Class)
	assert.Equal(t, c.Derived.RealizedLevel, p.RealizedLevel)
}

// ADR 0045 step 3.2 (Q4): an unknown class has no entry; there is no fallback
// to the character's level. A frozen entry's level is its left-at level.
func TestView_Step3_EntryOfAnUnknownClassIsAbsent(t *testing.T) {
	v := viewtest.Multi(viewtest.WithFrozen("fighter", 9), viewtest.WithEntry("mage", 7))

	_, ok := v.Entry("thief")
	assert.False(t, ok)
	_, ok = v.Level("thief")
	assert.False(t, ok)
	level, ok := v.Level("fighter")
	assert.True(t, ok)
	assert.Equal(t, 9, level)
	assert.Equal(t, []string{"mage"}, []string{v.Active()[0].Class})
	assert.Len(t, v.Active(), 1)
}

// ADR 0045 step 6.1: a service's test builds a view with no Derive and states
// the figures it wants; the builder computes none.
func TestViewtest_Step6_ABuilderViewNeedsNoDerive(t *testing.T) {
	v := viewtest.Single("fighter", 5, viewtest.WithScores(17, 14, 15, 10, 12, 9), viewtest.WithPSP(8, 20))
	v.THAC0 = 16

	assert.Equal(t, 16, v.THAC0)
	assert.Equal(t, 14, v.Dex)
	assert.Equal(t, 20, v.PSPMax)
	assert.Equal(t, &charactersvc.PSPPool{Current: 8, Max: 20}, v.PSP)
	assert.Equal(t, "fighter", v.Primary().Class)
	assert.Equal(t, 5, v.CasterLevel())
	assert.Equal(t, skills.Classes{{Class: "fighter", Level: 5}}, v.SkillClasses())
}

// ADR 0045 step 6.4: a builder-made view and a Derive-made one have the same
// shape: the entries' structure (class, state, left-at level, the order and
// what the helpers answer), not the figures.
func TestViewtest_Step6_BuilderMatchesDerivedShape(t *testing.T) {
	c := multi("human", entry("fighter", 5, 16_000))
	c.Derived = derived(t, c)
	derivedView := charactersvc.BuildView(*c, c.Derived)
	built := viewtest.Single("fighter", 5)

	require.Len(t, built.Entries, len(derivedView.Entries))
	for i := range built.Entries {
		assert.Equal(t, derivedView.Entries[i].Class, built.Entries[i].Class)
		assert.Equal(t, derivedView.Entries[i].State, built.Entries[i].State)
		assert.Equal(t, derivedView.Entries[i].LeftAtLevel, built.Entries[i].LeftAtLevel)
		assert.Equal(t, derivedView.Entries[i].RealizedLevel, built.Entries[i].RealizedLevel)
	}
	assert.Equal(t, derivedView.Primary().Class, built.Primary().Class)
	assert.Equal(t, derivedView.SkillClasses(), built.SkillClasses())
	assert.Len(t, built.Active(), len(derivedView.Active()))
}

// ADR 0045 step 3.1 on the stored form: classes.Primary and the derivation's
// firstActive share classes.PrimaryIndex.
func TestPrimaryIndex_Step3_FirstActiveOrNone(t *testing.T) {
	assert.Equal(t, 1, classes.PrimaryIndex([]classes.Entry{frozen("fighter", 9, 1), entry("mage", 10, 2)}))
	assert.Equal(t, -1, classes.PrimaryIndex([]classes.Entry{frozen("fighter", 9, 1)}))
	e, ok := classes.Find([]classes.Entry{frozen("fighter", 9, 1), entry("mage", 10, 2)}, "mage")
	assert.True(t, ok)
	assert.Equal(t, 10, e.Level())
}

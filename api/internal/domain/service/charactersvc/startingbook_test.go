package charactersvc_test

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// spellCatalogue is a spell catalogue by id.
type spellCatalogue map[string]spellbook.Spell

func (c spellCatalogue) Spell(id string) (spellbook.Spell, bool) {
	s, ok := c[id]
	return s, ok
}

func (c spellCatalogue) WizardSpells() []charactersvc.SpellOption {
	out := []charactersvc.SpellOption{}
	for id, s := range c {
		if s.Wizard {
			out = append(out, charactersvc.SpellOption{ID: id, Level: s.Level})
		}
	}
	slices.SortFunc(out, func(a, b charactersvc.SpellOption) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// StartingSpells are the starting spellbook seed's pre-ticked spells (seeds/starting_spellbook.json).
func (c spellCatalogue) StartingSpells(class string) []string {
	if class == "mage" {
		return []string{"read_magic", "detect_magic"}
	}
	return nil
}

// wizardCatalogue holds wizard spells of levels 1 to 9 (PHB) and a priest spell.
func wizardCatalogue() spellCatalogue {
	w := func(level int) spellbook.Spell { return spellbook.Spell{Level: level, Wizard: true} }
	return spellCatalogue{
		"read_magic": w(1), "detect_magic": w(1), "sleep": w(1), "light": w(1), "magic_missile": w(1),
		"mirror_image": w(2), "fireball": w(3), "stoneskin": w(4), "cloudkill": w(5), "death_fog": w(6),
		"incendiary_cloud": w(8), "meteor_swarm": w(9),
		"bless": {Level: 1},
	}
}

// bookWrites records the spellbook entries written, by character.
type bookWrites struct {
	entries map[int64][]spellbook.Entry
}

func (b *bookWrites) AddEntry(_ context.Context, id int64, e spellbook.Entry) error {
	if b.entries == nil {
		b.entries = map[int64][]spellbook.Entry{}
	}
	b.entries[id] = append(b.entries[id], e)
	return nil
}

// newBookEnv is newEnv whose rules answer PHB Table 4's row intRow for any Intelligence.
func newBookEnv(t *testing.T, intRow ruletablesvc.IntelligenceRow) *env {
	t.Helper()
	e := newEnv(t)
	e.rules = mocks.NewMockRules(t)
	withDefinitions(t, e.rules)
	e.rules.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
	e.rules.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
	e.rules.EXPECT().IntelligenceFor(mock.Anything).Return(intRow, nil).Maybe()
	e.rules.EXPECT().RogueSkills(skillSet("bard"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Armour: e.armour, HP: e.hp,
		Spells: e.spells, Book: e.book, FreeSpells: e.free,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// expectClassAt answers the derivation of class at xp, as often as asked: the level, and the
// class's spell slots there (PHB Table 21 for a mage, Table 32 for a bard).
func expectClassAt(e *env, class string, xp, level int, slots map[int]int) {
	group, _ := e.rules.ClassGroup(class) // the fixture's definitions (newRules)
	e.rules.EXPECT().RaceAllows(mock.Anything, class).Return(true, nil).Maybe()
	e.rules.EXPECT().LevelFor(class, xp).Return(level, nil).Maybe()
	e.rules.EXPECT().XPThreshold(class, level+1).Return(xp+1, nil).Maybe()
	e.rules.EXPECT().THAC0For(group, level).Return(20, nil).Maybe()
	e.rules.EXPECT().SavesFor(group, level).Return(ruletablesvc.SavingThrowRow{}, nil).Maybe()
	switch class {
	case "mage":
		e.rules.EXPECT().SpellsPerDay(classDef("mage"), level, mock.Anything).Return(slots, nil).Maybe()
	case "bard":
		e.rules.EXPECT().SpellsPerDay(classDef("bard"), level, mock.Anything).Return(slots, nil).Maybe()
	}
}

// Table 21's slots of a mage at levels 1, 9 and 18.
var (
	mage1Slots  = map[int]int{1: 1}
	mage9Slots  = map[int]int{1: 4, 2: 3, 3: 3, 4: 2, 5: 1}
	mage18Slots = map[int]int{1: 5, 2: 5, 3: 5, 4: 5, 5: 4, 6: 3, 7: 2, 8: 2, 9: 1}
)

// elminster is a valid new human mage at xp.
func elminster(xp int, spells ...string) charactersvc.NewCharacter {
	return charactersvc.NewCharacter{
		Details: identity(), Name: "Elminster", Class: "mage", Race: "human",
		Str: 9, Dex: 12, Con: 11, Int: 14, Wis: 12, Cha: 10, HPMax: 3, XP: xp, Spellbook: spells,
	}
}

// expectCreated answers the owners' writes of a created character 7.
func expectCreated(e *env) {
	created := e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).Return(7, nil).Once()
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(7)).Return(nil).Once().NotBefore(created)
	e.sessions.EXPECT().ForgetUser(ownerID).Once()
}

func starting(keys ...string) []spellbook.Entry {
	out := []spellbook.Entry{}
	for _, k := range keys {
		out = append(out, spellbook.Entry{Key: k, Origin: spellbook.OriginStarting})
	}
	return out
}

// ADR 0030 step 20.7: a mage created with a starting spellbook has each spell written in his
// book, in the order entered, with the origin starting (step 20.5), in the creation's one
// transaction. The service has no dice port, so no die can be rolled (the wiring's test counts
// them); left empty (the player unticked the pre-ticked spells), nothing is written.
func TestCreate_Step20_7_StartingSpellsWithoutADie(t *testing.T) {
	e := newBookEnv(t, int14)
	expectClassAt(e, "mage", 0, 1, mage1Slots)
	expectCreated(e)
	id, err := e.svc.Create(context.Background(), ownerID, elminster(0, "detect_magic", "sleep"))
	require.NoError(t, err)
	assert.Equal(t, starting("detect_magic", "sleep"), e.book.entries[id])
	assert.Equal(t, 1, e.tx.n, "one transaction")

	e = newBookEnv(t, int14)
	expectClassAt(e, "mage", 0, 1, mage1Slots)
	expectCreated(e)
	_, err = e.svc.Create(context.Background(), ownerID, elminster(0))
	require.NoError(t, err)
	assert.Empty(t, e.book.entries, "unticked: the book starts empty")
}

// spellRefusal is a refusal of the starting spell id.
func spellRefusal(id, code string) error {
	return &domain.FieldError{Field: charactersvc.FieldSpellbook + "." + id, Message: code}
}

// ADR 0030 step 20.7: each limit of step 20.1, read at the starting level, refuses the spell by
// its field and code, in the draft and at Create (before anything is written): a level 1 mage
// (Table 21: 1st level only) may not enter a 2nd-level spell; a level 9 mage may enter a 5th-level
// one but not a 6th; a level 18 mage with Intelligence 14 (Table 4: 7th level) may not enter an
// 8th-level one; Intelligence 9 (Table 4: 6 spells per level) refuses the 7th 1st-level spell; a
// priest spell, an unknown id and a spell entered twice are refused.
func TestCreate_Step20_7_LimitsAtTheStartingLevel(t *testing.T) {
	int9 := ruletablesvc.IntelligenceRow{IntMin: 9, IntMax: 9, MaxSpellLevel: 4, LearnChance: 35, MaxSpells: 6}
	for _, tt := range []struct {
		name   string
		xp     int
		level  int
		slots  map[int]int
		intRow ruletablesvc.IntelligenceRow
		spells []string
		want   error
	}{
		{
			"level 1 mage, 2nd-level spell", 0, 1, mage1Slots, int14,
			[]string{"sleep", "mirror_image"},
			spellRefusal("mirror_image", charactersvc.CodeSpellLevelAboveClass),
		},
		{
			"level 9 mage, 6th-level spell", 40_000, 9, mage9Slots, int14,
			[]string{"cloudkill", "death_fog"},
			spellRefusal("death_fog", charactersvc.CodeSpellLevelAboveClass),
		},
		{
			"level 18 mage, Int 14, 8th-level spell", 3_000_000, 18, mage18Slots, int14,
			[]string{"incendiary_cloud"},
			spellRefusal("incendiary_cloud", charactersvc.CodeSpellLevelAboveIntelligence),
		},
		{
			"Int 9, seven 1st-level spells", 0, 1, mage1Slots, int9,
			[]string{"read_magic", "detect_magic", "sleep", "light", "magic_missile", "x1", "x2"},
			nil,
		},
		{
			"level 9 mage, Int 9, 5th-level spell", 40_000, 9, mage9Slots, int9,
			[]string{"stoneskin", "cloudkill"},
			spellRefusal("cloudkill", charactersvc.CodeSpellLevelAboveIntelligence),
		},
		{
			"a priest spell", 0, 1, mage1Slots, int14,
			[]string{"bless"},
			spellRefusal("bless", charactersvc.CodeNotAWizardSpell),
		},
		{
			"an unknown spell", 0, 1, mage1Slots, int14,
			[]string{"nope"},
			spellRefusal("nope", charactersvc.CodeUnknown),
		},
		{
			"twice the same spell", 0, 1, mage1Slots, int14,
			[]string{"sleep", "sleep"},
			spellRefusal("sleep", charactersvc.CodeAlreadyInSpellbook),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newBookEnv(t, tt.intRow)
			want := tt.want
			if want == nil { // the per-level maximum: six 1st-level spells, the seventh refused
				for _, id := range []string{"x1", "x2"} {
					e.spells[id] = spellbook.Spell{Level: 1, Wizard: true}
				}
				want = spellRefusal("x2", charactersvc.CodeSpellbookLevelFull)
			}
			expectClassAt(e, "mage", tt.xp, tt.level, tt.slots)
			n := elminster(tt.xp, tt.spells...)

			d, err := e.svc.Draft(context.Background(), n, nil)
			require.NoError(t, err)
			assert.Equal(t, []error{want}, d.Refusals, "the draft's refusal")

			_, err = e.svc.Create(context.Background(), ownerID, n)
			assert.Equal(t, want, err, "Create's refusal")
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Zero(t, e.tx.n, "nothing written")
			assert.Empty(t, e.book.entries)
		})
	}
}

// ADR 0030 step 20.7: the draft answers the starting spellbook's limits at the starting level and
// the spells entered by spell level: a level 9 mage (Table 21: up to 5th level) with Intelligence
// 14 (Table 4: 7th level, 9 per level) enters spells of levels 1 to 5; a refused spell counts at
// no level, and every refusal is listed by spell, in the order entered.
func TestDraft_Step20_7_RefusesBySpellAndCode(t *testing.T) {
	e := newBookEnv(t, int14)
	expectClassAt(e, "mage", 40_000, 9, mage9Slots)
	d, err := e.svc.Draft(context.Background(),
		elminster(40_000, "read_magic", "bless", "sleep", "cloudkill", "meteor_swarm", "sleep", "fireball"), nil)
	require.NoError(t, err)
	assert.Equal(t, &charactersvc.StartingSpellbook{
		ClassMaxSpellLevel: 5, IntMaxSpellLevel: 7, MaxSpellsPerLevel: 9,
		Levels: []charactersvc.SpellLevelCount{{Level: 1, Count: 2}, {Level: 2}, {Level: 3, Count: 1}, {Level: 4}, {Level: 5, Count: 1}},
	}, d.Spellbook)
	assert.Equal(t, []error{
		spellRefusal("bless", charactersvc.CodeNotAWizardSpell),
		spellRefusal("meteor_swarm", charactersvc.CodeSpellLevelAboveClass),
		spellRefusal("sleep", charactersvc.CodeAlreadyInSpellbook),
	}, d.Refusals)
}

// ADR 0030 step 20.7 for a bard (PHB Table 32): at level 1 he has no spell level, so the draft
// offers none and refuses a 1st-level spell; at level 2 he may enter 1st-level spells, not 2nd.
// He has nothing pre-ticked (the options).
func TestDraft_Step20_7_ABardAtLevel1HasNoSpellLevel(t *testing.T) {
	e := newBookEnv(t, int14)
	expectClassAt(e, "bard", 0, 1, map[int]int{})
	n := lyra()
	n.Spellbook = []string{"sleep"}
	d, err := e.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	require.NotNil(t, d.Spellbook)
	assert.Zero(t, d.Spellbook.ClassMaxSpellLevel)
	assert.Empty(t, d.Spellbook.Levels, "no spell level to enter")
	assert.Equal(t, []error{spellRefusal("sleep", charactersvc.CodeSpellLevelAboveClass)}, d.Refusals)

	e = newBookEnv(t, int14)
	expectClassAt(e, "bard", 2_500, 2, map[int]int{1: 1})
	n = lyra()
	n.XP, n.Spellbook = 2_500, []string{"sleep", "mirror_image"}
	d, err = e.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.SpellLevelCount{{Level: 1, Count: 1}}, d.Spellbook.Levels)
	assert.Equal(t, []error{spellRefusal("mirror_image", charactersvc.CodeSpellLevelAboveClass)}, d.Refusals)
}

// ADR 0030 step 20.7: a class without a spellbook enters none: a fighter's, a thief's or a cleric's spellbook is refused on
// the spellbook field, by the draft and by Create, and the draft answers no spellbook.
func TestCreate_Step20_7_RefusedForAClassWithoutASpellbook(t *testing.T) {
	for _, class := range []string{"fighter", "thief", "cleric"} {
		t.Run(class, func(t *testing.T) {
			e := newBookEnv(t, int14)
			expectClassAt(e, class, 0, 1, nil)
			e.rules.EXPECT().SpellsPerDay(classDef("cleric"), mock.Anything, mock.Anything).Return(map[int]int{}, nil).Maybe()
			e.rules.EXPECT().RogueSkills(skillSet("thief"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
			e.rules.EXPECT().BackstabFor(mock.Anything).Return(2, nil).Maybe()
			e.rules.EXPECT().SpellFailureFor(mock.Anything).Return(0, nil).Maybe()
			e.rules.EXPECT().TurningAt(mock.Anything).Return(nil, nil).Maybe()
			n := elminster(0, "sleep")
			n.Class, n.Str, n.Dex, n.Wis = class, 12, 12, 12
			want := &domain.FieldError{Field: charactersvc.FieldSpellbook, Message: charactersvc.CodeNoSpellbook}

			d, err := e.svc.Draft(context.Background(), n, nil)
			require.NoError(t, err)
			assert.Nil(t, d.Spellbook)
			assert.Equal(t, []error{want}, d.Refusals)

			_, err = e.svc.Create(context.Background(), ownerID, n)
			assert.Equal(t, want, err)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
}

// ADR 0030 step 20.7: the creation options offer a mage's starting spellbook with Read Magic and
// Detect Magic pre-ticked (DMG p. 40), as the starting spellbook's data has them (the port; the
// seed's check against the catalogue is spellbookadpt's), a bard's with nothing, no spellbook to
// the other classes, and the catalogue's wizard spells by level.
func TestOptions_Step20_7_ReadMagicAndDetectMagicPreTickedForAMage(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(e)
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	books := map[string]*charactersvc.SpellbookOption{}
	for _, c := range got.Classes {
		books[c.Class] = c.Spellbook
	}
	assert.Equal(t, map[string]*charactersvc.SpellbookOption{
		"mage":    {StartingSpells: []string{"read_magic", "detect_magic"}},
		"bard":    {StartingSpells: []string{}},
		"fighter": nil, "cleric": nil, "thief": nil, "psionicist": nil,
	}, books)
	assert.Equal(t, e.spells.WizardSpells(), got.WizardSpells)
	assert.Equal(t, charactersvc.SpellOption{ID: "detect_magic", Level: 1}, got.WizardSpells[0])
}

// ADR 0030 steps 20.1 and 20.7: the two class maximums stay apart. A level 1 mage's derived
// spellbook keeps the class table's highest spell level (9th: what Learn and Ajouter check), while
// his starting spellbook's is the highest of his slots at the starting level (1st), so a 2nd-level
// spell is refused at creation only.
func TestDraft_Step20_7_StartingClassMaxIsTheSlotsNotTheTable(t *testing.T) {
	e := newBookEnv(t, int14)
	expectClassAt(e, "mage", 0, 1, mage1Slots)
	d, err := e.svc.Draft(context.Background(), elminster(0, "mirror_image"), nil)
	require.NoError(t, err)
	require.NotNil(t, d.Character)
	assert.Equal(t, 9, d.Character.Derived.Spellbook.ClassMaxSpellLevel, "the table's, for Learn")
	assert.Equal(t, 1, d.Spellbook.ClassMaxSpellLevel, "the slots', at the starting level")
	assert.Equal(t, []charactersvc.SpellLevelCount{{Level: 1}}, d.Spellbook.Levels, "only the 1st level offered")
	assert.Equal(t, []error{spellRefusal("mirror_image", charactersvc.CodeSpellLevelAboveClass)}, d.Refusals)
}

// ADR 0030 step 20.7 with Table 4's "All": Intelligence 19 has no maximum of spells per level, so
// a starting book takes nineteen 1st-level spells, more than the 18 of Intelligence 18.
func TestCreate_Step20_7_Intelligence19HasNoMaximum(t *testing.T) {
	int19 := ruletablesvc.IntelligenceRow{IntMin: 19, IntMax: 19, MaxSpellLevel: 9, LearnChance: 95, NoMaximum: true}
	e := newBookEnv(t, int19)
	ids := []string{}
	for i := range 19 {
		id := fmt.Sprintf("first_%02d", i)
		e.spells[id] = spellbook.Spell{Level: 1, Wizard: true}
		ids = append(ids, id)
	}
	expectClassAt(e, "mage", 0, 1, mage1Slots)
	expectCreated(e)
	n := elminster(0, ids...)
	n.Int = 19
	id, err := e.svc.Create(context.Background(), ownerID, n)
	require.NoError(t, err)
	assert.Len(t, e.book.entries[id], 19)
}

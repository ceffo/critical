// The adapter runs on a mocked Querier: the tests assert the operations
// called, their arguments (the filter, the keys, the text ids read) and how
// rows map to spells; no database, no SQL text (ADR 0025 step 6.2). What only
// the SQL decides (rows without a key never served, the class compared
// case-insensitively, the text lookup through the primary key) is checked by
// sqlc at generate time and by running the app (step 7).
package spelldal_test

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/spelldal/queries"
	"github.com/ceffo/critical/api/internal/adapters/spelldal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

const charID = int64(3)

var errBoom = errors.New("boom")

func mocked(t *testing.T) (*spelldal.DAL, *mocks.MockQuerier) {
	t.Helper()
	q := mocks.NewMockQuerier(t)
	return spelldal.New(q), q
}

func id(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
func str(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

// texts answers each TextsByID call with the rows of the ids asked, from
// stored, and records the ids of every call.
type texts struct {
	stored map[int64]localized.Text
	asked  [][]int64
}

func (x *texts) expect(q *mocks.MockQuerier) {
	q.EXPECT().TextsByID(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ids []int64) ([]queries.LocalizedText, error) {
		x.asked = append(x.asked, slices.Clone(ids))
		var out []queries.LocalizedText
		for _, i := range ids {
			for lang, body := range x.stored[i] {
				out = append(out, queries.LocalizedText{TextID: i, Lang: lang, Body: body})
			}
		}
		return out, nil
	})
}

func (x *texts) all() []int64 {
	var out []int64
	for _, a := range x.asked {
		out = append(out, a...)
	}
	return out
}

// spell is a served spells row with stored stats, its name under text key
// nameID and its school under nameID+1.
func spell(key, class string, level, nameID int64) queries.Spell {
	return queries.Spell{
		ID: nameID, Key: str(key), Class: class, Level: level, Name: id(nameID), School: id(nameID + 1), Stats: str(`{}`),
	}
}

func names(spells []spellsvc.Spell) []string {
	out := make([]string, len(spells))
	for i, s := range spells {
		out[i] = s.Class + ":" + s.Name.Default()
	}
	return out
}

func TestSlots_MissingRowIsEmpty(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().GetSpellSlots(ctx, charID).Return("", sql.ErrNoRows).Once()
	got, err := dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	q.EXPECT().GetSpellSlots(ctx, charID).Return("", nil).Once()
	got, err = dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.SpellSlot{}, got)

	q.EXPECT().GetSpellSlots(ctx, charID).Return("", errBoom).Once()
	_, err = dal.GetSlots(ctx, charID)
	require.ErrorIs(t, err, errBoom)

	q.EXPECT().GetSpellSlots(ctx, charID).Return("{bad", nil).Once()
	_, err = dal.GetSlots(ctx, charID)
	require.Error(t, err)
}

// The on-disk format of the slots, read and written.
func TestSlots_ReadsStoredJSON(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().GetSpellSlots(ctx, charID).Return(
		`[{"level":1,"prepared_spell_name":"Sleep","used":false},{"level":2,"prepared_spell_id":"invisibility","prepared_spell_name":"Invisibility","used":true,"extra":true}]`, nil).Once()
	got, err := dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, SpellID: "invisibility", PreparedSpellName: "Invisibility", Used: true, Extra: true},
	}, got)
}

// Save writes the whole list as the character's one row; what it writes
// reads back as it was, a slot's entry class included (ADR 0042 step 14: the
// JSON blob keeps "class").
func TestSlots_RoundTripAndOverwrite(t *testing.T) {
	ctx := context.Background()
	slots := []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, SpellID: "invisibility", PreparedSpellName: "Invisibility", Used: true, Extra: true},
		{Level: 3, SpellID: "fireball", PreparedSpellName: "Fireball", Used: true, Class: "mage"},
	}
	dal, q := mocked(t)
	var blob string
	q.EXPECT().UpsertSpellSlots(ctx, mock.Anything).RunAndReturn(func(_ context.Context, p queries.UpsertSpellSlotsParams) error {
		assert.Equal(t, charID, p.CharacterID)
		blob = p.Slots
		return nil
	}).Once()
	require.NoError(t, dal.SaveSlots(ctx, charID, slots))
	assert.Contains(t, blob, `"class":"mage"`)
	q.EXPECT().GetSpellSlots(ctx, charID).Return(blob, nil).Once()
	got, err := dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, slots, got)

	q.EXPECT().UpsertSpellSlots(ctx, mock.Anything).Return(errBoom).Once()
	require.ErrorIs(t, dal.SaveSlots(ctx, charID, slots), errBoom)
}

// The stored columns of the filter go to the query as they are: the class
// (empty is any), the levels, and "any level" only when none is given (level
// 0 is a level). The names, school groups and stats are not the DAL's.
func TestSearch_PassesTheColumnFilters(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		f    spellsvc.Filter
		want queries.SearchSpellsParams
	}{
		{"no filter", spellsvc.Filter{}, queries.SearchSpellsParams{AnyLevel: 1, Levels: []int64{}}},
		{"class is lowered (case-insensitive match on every engine)", spellsvc.Filter{Class: "WIZARD"}, queries.SearchSpellsParams{Class: "wizard", AnyLevel: 1, Levels: []int64{}}},
		{"mixed-case class is lowered", spellsvc.Filter{Class: "Priest"}, queries.SearchSpellsParams{Class: "priest", AnyLevel: 1, Levels: []int64{}}},
		{"several levels", spellsvc.Filter{Levels: []int{3, 1}}, queries.SearchSpellsParams{Levels: []int64{3, 1}}},
		{"level 0 is a level, not 'any'", spellsvc.Filter{Levels: []int{0}}, queries.SearchSpellsParams{Levels: []int64{0}}},
		{"class and levels combined", spellsvc.Filter{Class: "wizard", Levels: []int{1, 2}}, queries.SearchSpellsParams{Class: "wizard", Levels: []int64{1, 2}}},
		{
			"name, school and stats are not the DAL's",
			spellsvc.Filter{Query: "zzz", School: spellsvc.SchoolOther, Save: spellstats.SaveHalf},
			queries.SearchSpellsParams{AnyLevel: 1, Levels: []int64{}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dal, q := mocked(t)
			q.EXPECT().SearchSpells(ctx, c.want).Return([]queries.Spell{}, nil).Once()
			q.EXPECT().TextsByID(ctx, []int64{}).Return(nil, nil).Maybe()
			got, err := dal.Search(ctx, c.f)
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Empty(t, got)
		})
	}
}

// Search orders by level, then fallback-language name, then class, whatever
// the rows' order; the same name in two classes is two spells.
func TestSearch_OrdersByLevelNameThenClass(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, mock.Anything).Return([]queries.Spell{
		spell("fireball", "wizard", 3, 10),
		spell("sleep", "wizard", 1, 20),
		spell("invisibility", "bard", 2, 30),
		spell("sleep_bard", "bard", 1, 40),
		spell("charm_person", "bard", 1, 50),
	}, nil).Once()
	x := &texts{stored: map[int64]localized.Text{
		10: localized.Of("Fireball"), 20: localized.Of("Sleep"), 30: localized.Of("Invisibility"),
		40: localized.Of("Sleep"), 50: localized.Of("Charm Person"),
	}}
	x.expect(q)
	got, err := dal.Search(ctx, spellsvc.Filter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"bard:Charm Person", "bard:Sleep", "wizard:Sleep", "bard:Invisibility", "wizard:Fireball"}, names(got))
}

// Search reads the texts it filters and ranks on (every language's name, the
// school, the sphere) in one batched lookup, never the description or the
// stat texts of a spell with stored stats; Details and Describe read every
// text, in every language stored.
func TestSearchAndDetails_MapFields(t *testing.T) {
	ctx := context.Background()
	row := queries.Spell{
		ID: 1, Key: str("fireball"), Class: "wizard", Level: 3, Source: str("PHB"),
		Name: id(1), School: id(2), Sphere: id(3), Range: id(4), Description: id(5), Stats: str(`{}`),
	}
	stored := map[int64]localized.Text{
		1: {"en": "Fireball", "fr": "Boule de feu"}, 2: localized.Of("Evocation"), 3: localized.Of("Elemental"),
		4: localized.Of("10 yds."), 5: {"en": "desc", "fr": "texte"},
	}

	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, queries.SearchSpellsParams{Levels: []int64{3}}).Return([]queries.Spell{row}, nil).Once()
	x := &texts{stored: stored}
	x.expect(q)
	got, err := dal.Search(ctx, spellsvc.Filter{Levels: []int{3}})
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.Spell{{
		ID: "fireball", Name: localized.Text{"en": "Fireball", "fr": "Boule de feu"}, Class: "wizard", Level: 3,
		School: localized.Of("Evocation"), Sphere: localized.Of("Elemental"), Source: "PHB",
	}}, got)
	assert.Equal(t, [][]int64{{1, 2, 3}}, x.asked, "name, school and sphere only, in one lookup")

	full := spellsvc.Spell{
		ID: "fireball", Name: localized.Text{"en": "Fireball", "fr": "Boule de feu"}, Class: "wizard", Level: 3,
		School: localized.Of("Evocation"), Sphere: localized.Of("Elemental"), Range: localized.Of("10 yds."),
		Description: localized.Text{"en": "desc", "fr": "texte"}, Source: "PHB",
	}
	dal, q = mocked(t)
	q.EXPECT().SpellsByKey(ctx, []string{"nope", "fireball"}).Return([]queries.Spell{row}, nil).Once()
	x = &texts{stored: stored}
	x.expect(q)
	details, err := dal.Details(ctx, []string{"nope", "fireball"})
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.Spell{full}, details, "an unknown id is skipped")
	assert.ElementsMatch(t, []int64{1, 2, 3, 4, 5}, x.all())
	assert.Len(t, x.asked, 1, "one batched lookup")

	q.EXPECT().SpellsByKey(ctx, []string{"fireball"}).Return([]queries.Spell{row}, nil).Once()
	fb, ok, err := dal.Describe(ctx, "fireball")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, full, fb)
}

// Details answers in the order of the ids asked, whatever the rows' order;
// no id asks nothing.
func TestDetails_KeepsTheOrderOfIDs(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().SpellsByKey(ctx, mock.Anything).Return([]queries.Spell{
		spell("charm_person", "bard", 1, 10), spell("sleep_bard", "bard", 1, 20), spell("fireball", "wizard", 3, 30),
	}, nil).Once()
	x := &texts{stored: map[int64]localized.Text{10: localized.Of("Charm Person"), 20: localized.Of("Sleep"), 30: localized.Of("Fireball")}}
	x.expect(q)
	got, err := dal.Details(ctx, []string{"fireball", "sleep_bard", "charm_person"})
	require.NoError(t, err)
	assert.Equal(t, []string{"wizard:Fireball", "bard:Sleep", "bard:Charm Person"}, names(got))

	none, err := dal.Details(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

// Describe answers to the stored catalogue key: no row holding it is not
// found; a failing read is an error.
func TestDescribe(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().SpellsByKey(ctx, []string{"a_new_name"}).Return([]queries.Spell{}, nil).Once()
	q.EXPECT().TextsByID(ctx, []int64{}).Return(nil, nil).Maybe()
	_, ok, err := dal.Describe(ctx, "a_new_name")
	require.NoError(t, err)
	assert.False(t, ok)

	q.EXPECT().SpellsByKey(ctx, []string{"light"}).Return(nil, errBoom).Once()
	_, _, err = dal.Describe(ctx, "light")
	require.ErrorIs(t, err, errBoom)
}

// A row without stats JSON (or with garbage) is typed from its
// fallback-language stat texts, which Search then reads too; Details types
// it the same way.
func TestSearch_RowWithoutStatsIsParsedFromRawText(t *testing.T) {
	ctx := context.Background()
	a := queries.Spell{ID: 1, Key: str("a"), Class: "wizard", Level: 1, Name: id(1), Range: id(2), SavingThrow: id(3)}
	b := queries.Spell{ID: 2, Key: str("b"), Class: "wizard", Level: 2, Name: id(4), Range: id(5), SavingThrow: id(6), Stats: str("{not json")}
	stored := map[int64]localized.Text{
		1: localized.Of("A"), 2: localized.Of("10 yds./level"), 3: localized.Of("1/2"),
		4: localized.Of("B"), 5: localized.Of("20 yds."), 6: localized.Of("Neg."),
	}
	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, mock.Anything).Return([]queries.Spell{a, b}, nil).Once()
	x := &texts{stored: stored}
	x.expect(q)
	got, err := dal.Search(ctx, spellsvc.Filter{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 150.0, got[0].Stats.At(15).Range.Amount)
	assert.Equal(t, []string{"half"}, got[0].Stats.Save.Kinds)
	assert.Equal(t, []string{"negates"}, got[1].Stats.Save.Kinds)
	assert.ElementsMatch(t, []int64{1, 2, 3, 4, 5, 6}, x.all())

	q.EXPECT().SpellsByKey(ctx, []string{"a"}).Return([]queries.Spell{a}, nil).Once()
	details, err := dal.Details(ctx, []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, got[0].Stats, details[0].Stats)
}

// The stored typed stats are served as stored.
func TestSearch_ServesTheStoredStats(t *testing.T) {
	ctx := context.Background()
	row := spell("magic_missile", "wizard", 1, 1)
	row.Stats = str(`{"range":{"kind":"distance","raw":"60 yds. + 10 yds./level","base":60,"perLevel":10,"unit":"yd"}}`)
	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, mock.Anything).Return([]queries.Spell{row}, nil).Once()
	x := &texts{stored: map[int64]localized.Text{1: localized.Of("Magic Missile")}}
	x.expect(q)
	got, err := dal.Search(ctx, spellsvc.Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "60 yds. + 10 yds./level", got[0].Stats.Range.Raw)
	assert.Equal(t, [][]int64{{1, 2}}, x.asked, "no stat text read for stored stats")
}

// The text lookup is batched below SQLite's limit on bound parameters, and
// every batch's rows are kept.
func TestSearch_ReadsTheTextsInBatches(t *testing.T) {
	ctx := context.Background()
	rows := make([]queries.Spell, 2600)
	stored := map[int64]localized.Text{}
	for i := range rows {
		rows[i] = spell("s", "wizard", 1, int64(2*i+1))
		stored[int64(2*i+1)] = localized.Of("n")
	}
	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, mock.Anything).Return(rows, nil).Once()
	x := &texts{stored: stored}
	x.expect(q)
	got, err := dal.Search(ctx, spellsvc.Filter{})
	require.NoError(t, err)
	require.Len(t, x.asked, 2)
	assert.Equal(t, []int{5000, 200}, []int{len(x.asked[0]), len(x.asked[1])})
	for _, s := range got {
		require.Equal(t, "n", s.Name.Default())
	}
}

func TestSearch_Errors(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().SearchSpells(ctx, mock.Anything).Return(nil, errBoom).Once()
	_, err := dal.Search(ctx, spellsvc.Filter{})
	require.ErrorIs(t, err, errBoom)

	q.EXPECT().SearchSpells(ctx, mock.Anything).Return([]queries.Spell{spell("a", "wizard", 1, 1)}, nil).Once()
	q.EXPECT().TextsByID(ctx, mock.Anything).Return(nil, errBoom).Once()
	_, err = dal.Search(ctx, spellsvc.Filter{})
	require.ErrorIs(t, err, errBoom)
}

package spelldal_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open("file::memory:?cache=shared&mode=memory")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func seedCharacter(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('Test', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func TestSlots_MissingRowIsEmpty(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)

	got, err := spelldal.New(db).GetSlots(context.Background(), charID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSlots_RoundTripAndOverwrite(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := spelldal.New(db)
	ctx := context.Background()

	slots := []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, PreparedSpellName: "Invisibility", Used: true},
	}
	require.NoError(t, dal.SaveSlots(ctx, charID, slots))
	got, err := dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, slots, got)

	slots[0].Used = true
	require.NoError(t, dal.SaveSlots(ctx, charID, slots))
	got, err = dal.GetSlots(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, slots, got)
}

func TestSlots_PerCharacter(t *testing.T) {
	db := openDB(t)
	c1, c2 := seedCharacter(t, db), seedCharacter(t, db)
	dal := spelldal.New(db)
	ctx := context.Background()

	require.NoError(t, dal.SaveSlots(ctx, c1, []spellsvc.SpellSlot{{Level: 1, PreparedSpellName: "Sleep"}}))
	got, err := dal.GetSlots(ctx, c2)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// insertSpell writes a spells row with its texts as localized_texts rows
// (ADR 0023), under fresh keys; a column without a text stays NULL.
func insertSpell(t *testing.T, db *sqlite.DB, key any, class string, level int, stats any, texts map[string]localized.Text) {
	t.Helper()
	ctx := context.Background()
	cols := []string{"key", "class", "level", "stats"}
	args := []any{key, class, level, stats}
	for col, txt := range texts {
		var id int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(MAX(text_id), 0) + 1 FROM localized_texts`).Scan(&id))
		for lang, body := range txt {
			_, err := db.ExecContext(ctx, `INSERT INTO localized_texts (text_id, lang, body) VALUES (?, ?, ?)`, id, lang, body)
			require.NoError(t, err)
		}
		cols = append(cols, col)
		args = append(args, id)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO spells (`+strings.Join(cols, ", ")+`) VALUES (?`+
		strings.Repeat(", ?", len(cols)-1)+`)`, args...)
	require.NoError(t, err)
}

func seedSpells(t *testing.T, db *sqlite.DB) {
	t.Helper()
	rows := []struct {
		key, name, class string
		level            int
	}{
		{"sleep_bard", "Sleep", "bard", 1},
		{"charm_person", "Charm Person", "bard", 1},
		{"invisibility", "Invisibility", "bard", 2},
		{"sleep", "Sleep", "wizard", 1},
		{"fireball", "Fireball", "wizard", 3},
	}
	for _, r := range rows {
		insertSpell(t, db, r.key, r.class, r.level, nil, map[string]localized.Text{
			"name": localized.Of(r.name), "school": localized.Of("Enchantment"), "description": localized.Of("desc"),
		})
	}
}

func names(spells []spellsvc.Spell) []string {
	out := make([]string, len(spells))
	for i, s := range spells {
		out[i] = s.Class + ":" + s.Name.Default()
	}
	return out
}

func TestSearch(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)
	dal := spelldal.New(db)
	tests := []struct {
		name  string
		class string
		want  []string
	}{
		{"no filter, ordered by level then name", "", []string{"bard:Charm Person", "bard:Sleep", "wizard:Sleep", "bard:Invisibility", "wizard:Fireball"}},
		{"class filter", "wizard", []string{"wizard:Sleep", "wizard:Fireball"}},
		{"no match", "priest", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dal.Search(context.Background(), spellsvc.Filter{Class: tt.class})
			require.NoError(t, err)
			assert.Equal(t, tt.want, names(got))
		})
	}
}

// Search reads the texts it filters and ranks on (every language's name, the
// school, the sphere), never the description or the stat texts; Details and
// Describe read every text, in every language stored.
func TestSearchAndDetails_MapFields(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	insertSpell(t, db, "fireball", "wizard", 3, `{}`, map[string]localized.Text{
		"name": {"en": "Fireball", "fr": "Boule de feu"}, "school": localized.Of("Evocation"),
		"description": {"en": "desc", "fr": "texte"}, "range": localized.Of("10 yds."),
	})
	_, err := db.ExecContext(ctx, `UPDATE spells SET source = 'PHB'`)
	require.NoError(t, err)
	dal := spelldal.New(db)

	got, err := dal.Search(ctx, spellsvc.Filter{Levels: []int{3}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, spellsvc.Spell{
		ID: "fireball", Name: localized.Text{"en": "Fireball", "fr": "Boule de feu"}, Class: "wizard", Level: 3,
		School: localized.Of("Evocation"), Source: "PHB",
	}, got[0])

	full := spellsvc.Spell{
		ID: "fireball", Name: localized.Text{"en": "Fireball", "fr": "Boule de feu"}, Class: "wizard", Level: 3,
		School: localized.Of("Evocation"), Range: localized.Of("10 yds."),
		Description: localized.Text{"en": "desc", "fr": "texte"}, Source: "PHB",
	}
	details, err := dal.Details(ctx, []string{"nope", "fireball"})
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.Spell{full}, details, "an unknown id is skipped")
	fb, ok, err := dal.Describe(ctx, "fireball")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, full, fb)
}

// Details answers in the order of the ids asked, whatever the storage order.
func TestDetails_KeepsTheOrderOfIDs(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)
	got, err := spelldal.New(db).Details(context.Background(), []string{"fireball", "sleep_bard", "charm_person"})
	require.NoError(t, err)
	assert.Equal(t, []string{"wizard:Fireball", "bard:Sleep", "bard:Charm Person"}, names(got))
	none, err := spelldal.New(db).Details(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// ADR 0023 search cost: the text lookup reaches localized_texts through its
// primary key (text_id, lang), never by scanning the table, so loading the
// names never reads the descriptions.
func TestTextsQuery_SearchesThePrimaryKey(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	importFixture(t, db)
	query, args, err := spelldal.TextsQuery([]int64{1, 2, 3}).Build(ctx)
	require.NoError(t, err)
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		plan = append(plan, detail)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, plan)
	joined := strings.Join(plan, "; ")
	assert.Contains(t, joined, "SEARCH localized_texts USING PRIMARY KEY (text_id=?)", joined)
	assert.NotContains(t, joined, "SCAN localized_texts", joined)
}

// Rows without a key (older duplicates the catalogue import left in place)
// are never served, by Search or Describe.
func TestSearchAndDescribe_RowsWithoutAKeyAreNotServed(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	seedSpells(t, db)
	insertSpell(t, db, nil, "wizard", 3, nil, map[string]localized.Text{
		"name": localized.Of("Fireball"), "description": localized.Of("older duplicate"),
	})
	got, err := spelldal.New(db).Search(ctx, spellsvc.Filter{Levels: []int{3}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "fireball", got[0].ID)
	fb, ok, err := spelldal.New(db).Describe(ctx, "fireball")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, localized.Of("desc"), fb.Description)
}

// A row without stats JSON (or with garbage) is typed from its
// fallback-language stat texts, by Search and by Details.
func TestSearch_RowWithoutStatsIsParsedFromRawText(t *testing.T) {
	db := openDB(t)
	insertSpell(t, db, "a", "wizard", 1, nil, map[string]localized.Text{
		"name": localized.Of("A"), "range": localized.Of("10 yds./level"), "saving_throw": localized.Of("1/2"),
	})
	insertSpell(t, db, "b", "wizard", 2, "{not json", map[string]localized.Text{
		"name": localized.Of("B"), "range": localized.Of("20 yds."), "saving_throw": localized.Of("Neg."),
	})
	got, err := spelldal.New(db).Search(context.Background(), spellsvc.Filter{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 150.0, got[0].Stats.At(15).Range.Amount)
	assert.Equal(t, []string{"half"}, got[0].Stats.Save.Kinds)
	assert.Equal(t, []string{"negates"}, got[1].Stats.Save.Kinds)
	details, err := spelldal.New(db).Details(context.Background(), []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, got[0].Stats, details[0].Stats)
}

func importFixture(t *testing.T, db *sqlite.DB) catalogue.Catalogue {
	t.Helper()
	cat, err := catalogueadpt.Load(filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	require.NoError(t, db.ImportCatalogue(context.Background(), cat, nil))
	return cat
}

// The import stores the typed stats; the DAL returns them with the raw text,
// and the French texts when the catalogue has them.
func TestSearch_ImportedSpellsCarryTypedStatsAndFrench(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	importFixture(t, db)

	got, err := spelldal.New(db).Search(ctx, spellsvc.Filter{Class: "wizard", Levels: []int{1}})
	require.NoError(t, err)
	var mm spellsvc.Spell
	for _, s := range got {
		if s.Name.Default() == "Magic Missile" {
			mm = s
		}
	}
	require.Equal(t, "magic_missile", mm.ID)
	assert.Equal(t, localized.Text{"en": "Magic Missile", "fr": "Projectile magique"}, mm.Name)
	assert.Equal(t, "60 yds. + 10 yds./level", mm.Stats.Range.Raw)
	full, ok, err := spelldal.New(db).Describe(ctx, "magic_missile")
	require.NoError(t, err)
	require.True(t, ok)
	assert.NotEmpty(t, full.Description["fr"])
	assert.Equal(t, localized.Of("60 yds. + 10 yds./level"), full.Range)
	assert.Equal(t, spellstats.RangeDistance, mm.Stats.Range.Kind)
	assert.Equal(t, 210.0, mm.Stats.At(15).Range.Amount)

	fb, ok, err := spelldal.New(db).Describe(ctx, "fireball")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{spellstats.SaveHalf}, fb.Stats.Save.Kinds)

	armor, ok, err := spelldal.New(db).Describe(ctx, "armor")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, localized.Of("Armor"), armor.Name, "no French source: no fr entry, the reader falls back to English")
	assert.NotContains(t, armor.Description, "fr")
}

func TestSlots_ReadsStoredJSON(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)`, charID,
		`[{"level":1,"prepared_spell_name":"Sleep","used":false},{"level":2,"prepared_spell_name":"Invisibility","used":true}]`)
	require.NoError(t, err)

	got, err := spelldal.New(db).GetSlots(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.SpellSlot{
		{Level: 1, PreparedSpellName: "Sleep"},
		{Level: 2, PreparedSpellName: "Invisibility", Used: true},
	}, got)
}

func TestDescribe(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	// Describe answers to the stored catalogue key, never a key computed
	// from the name.
	for _, r := range []struct {
		key, name, class string
		level            int
	}{
		{"light_priest", "Light", "Priest", 1},
		{"light", "Light", "Wizard", 1},
		{"melfs_acid_arrow", "Melf's Acid Arrow", "Wizard", 1},
		{"bless", "Bless", "Priest", 1},
		{"twin_l4", "Twin", "Priest", 4},
		{"twin", "Twin", "Priest", 3},
		{"renamed_spell", "A New Name", "Wizard", 2},
	} {
		insertSpell(t, db, r.key, r.class, r.level, nil, map[string]localized.Text{
			"name": localized.Of(r.name), "description": localized.Of(r.class + " text"),
		})
	}
	d := spelldal.New(db)
	tests := []struct {
		id, wantName, wantClass string
		wantLevel               int
		found                   bool
	}{
		{"light", "Light", "Wizard", 1, true},
		{"light_priest", "Light", "Priest", 1, true},
		{"melfs_acid_arrow", "Melf's Acid Arrow", "Wizard", 1, true},
		{"bless", "Bless", "Priest", 1, true},
		{"twin", "Twin", "Priest", 3, true},
		{"twin_l4", "Twin", "Priest", 4, true},
		{"renamed_spell", "A New Name", "Wizard", 2, true},
		{"a_new_name", "", "", 0, false}, // the slug of the name is not the key
		{"no_such_spell", "", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got, ok, err := d.Describe(ctx, tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.found, ok)
			assert.Equal(t, tt.wantName, got.Name.Default())
			assert.Equal(t, tt.wantClass, got.Class)
			assert.Equal(t, tt.wantLevel, got.Level)
		})
	}
}

func TestSearch_ColumnFilters(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)
	_, err := db.ExecContext(context.Background(), `UPDATE localized_texts SET body = 'Evocation'
		WHERE text_id = (SELECT school FROM spells WHERE key = 'fireball')`)
	require.NoError(t, err)
	dal := spelldal.New(db)

	tests := []struct {
		name string
		f    spellsvc.Filter
		want []string
	}{
		{"level", spellsvc.Filter{Levels: []int{2}}, []string{"bard:Invisibility"}},
		{"several levels", spellsvc.Filter{Levels: []int{3, 1}}, []string{"bard:Charm Person", "bard:Sleep", "wizard:Sleep", "wizard:Fireball"}},
		{"level 0 is a level, not 'any'", spellsvc.Filter{Levels: []int{0}}, []string{}},
		{"class is case-insensitive", spellsvc.Filter{Class: "WIZARD"}, []string{"wizard:Sleep", "wizard:Fireball"}},
		{"class and levels combined", spellsvc.Filter{Class: "wizard", Levels: []int{1, 2}}, []string{"wizard:Sleep"}},
		{"combined, nothing left", spellsvc.Filter{Class: "wizard", Levels: []int{2}}, []string{}},
		{"name and school are not the DAL's", spellsvc.Filter{Query: "zzz", School: spellsvc.SchoolOther}, []string{"bard:Charm Person", "bard:Sleep", "wizard:Sleep", "bard:Invisibility", "wizard:Fireball"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dal.Search(context.Background(), tt.f)
			require.NoError(t, err)
			assert.Equal(t, tt.want, names(got))
		})
	}
}

func TestSearch_SameNameInBothClassesComesBackAsTwoRows(t *testing.T) {
	db := openDB(t)
	seedSpells(t, db)
	insertSpell(t, db, "sleep_priest", "priest", 1, nil, map[string]localized.Text{"name": localized.Of("Sleep")})

	got, err := spelldal.New(db).Search(context.Background(), spellsvc.Filter{Levels: []int{1}})
	require.NoError(t, err)
	assert.Equal(t, []string{"bard:Charm Person", "bard:Sleep", "priest:Sleep", "wizard:Sleep"}, names(got))
}

// The real service over the real DAL and the fixture catalogue.
func TestSearch_FiltersCombineOverTheFixtureCatalogue(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	seeds := filepath.Join("..", "..", "..", "testdata", "seeds")
	require.NoError(t, db.Seed(ctx, seeds))
	cat := importFixture(t, db)
	dal := spelldal.New(db)
	book, err := spellbookadpt.New(seeds, cat)
	require.NoError(t, err)
	svc, err := spellsvc.New(dal, dal, book)
	require.NoError(t, err)

	all := spellsvc.PageRequest{PageSize: spellsvc.MaxPageSize}
	both, err := svc.Search(ctx, spellsvc.Filter{Query: "Detect Magic"}, all)
	require.NoError(t, err)
	require.Len(t, both.Items, 2)
	assert.ElementsMatch(t, []string{"Wizard", "Priest"}, []string{both.Items[0].Class, both.Items[1].Class})

	wiz, err := svc.Search(ctx, spellsvc.Filter{Query: "Detect Magic", Class: "wizard"}, all)
	require.NoError(t, err)
	require.Len(t, wiz.Items, 1)
	assert.Equal(t, "Wizard", wiz.Items[0].Class)

	half, err := svc.Search(ctx, spellsvc.Filter{Class: "wizard", Levels: []int{3}, Save: spellstats.SaveHalf}, all)
	require.NoError(t, err)
	got := []string{}
	for _, s := range half.Items {
		got = append(got, s.Name.Default())
	}
	assert.Contains(t, got, "Fireball")
	assert.Contains(t, got, "Lightning Bolt")
	assert.NotContains(t, got, "Haste", "Haste has no save")

	// The French name matches like the English one (ADR 0023 step 7), and the
	// page carries every text of its spells.
	fr, err := svc.Search(ctx, spellsvc.Filter{Query: "projectile"}, all)
	require.NoError(t, err)
	require.Len(t, fr.Items, 1)
	assert.Equal(t, "magic_missile", fr.Items[0].ID)
	assert.NotEmpty(t, fr.Items[0].Description["fr"])
	assert.NotEmpty(t, fr.Items[0].Range.Default())

	// Mine is exactly the fixture spellbook (all Wizard), in level-then-name order.
	mine, err := svc.Search(ctx, spellsvc.Filter{Class: "wizard", Mine: true}, all)
	require.NoError(t, err)
	gotIDs := []string{}
	for _, s := range mine.Items {
		gotIDs = append(gotIDs, s.ID)
	}
	wantIDs := []string{}
	for _, e := range book.All() {
		wantIDs = append(wantIDs, e.ID)
	}
	assert.ElementsMatch(t, wantIDs, gotIDs)
	assert.Equal(t, len(wantIDs), mine.Total)
}

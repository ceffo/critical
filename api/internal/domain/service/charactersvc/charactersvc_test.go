package charactersvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// tx runs the body in the caller's context and counts the transactions.
type tx struct{ n int }

func (t *tx) InTx(ctx context.Context, fn func(context.Context) error) error {
	t.n++
	return fn(ctx)
}

// wornArmour answers the Table 29 column of the body armour each character
// wears (ADR 0033 step 14), none when absent, and counts the reads.
type wornArmour struct {
	columns map[int64]string
	reads   int
}

func (w *wornArmour) WornSkillColumn(_ context.Context, id int64) (string, error) {
	w.reads++
	return w.columns[id], nil
}

// hpLedger answers the HP ledger's total of each character.
type hpLedger map[int64]int

func (h hpLedger) HPTotal(_ context.Context, id int64) (int, error) { return h[id], nil }

func (h hpLedger) PSPTotal(context.Context, int64) (int, error) { return 0, nil }

// freeSpells answers the levels each character took in the app and the free
// spells its spellbook holds (ADR 0030 step 20.8), and counts the reads.
type freeSpells struct {
	taken   map[int64][]int
	claimed map[int64][]spellbook.FreeSpell
	reads   int
}

// LevelsTaken are the taken levels, each in the first active entry (a record
// with no class).
func (f *freeSpells) LevelsTaken(_ context.Context, id int64) ([]charactersvc.TakenLevel, error) {
	f.reads++
	out := make([]charactersvc.TakenLevel, 0, len(f.taken[id]))
	for _, l := range f.taken[id] {
		out = append(out, charactersvc.TakenLevel{Level: l})
	}
	return out, nil
}

func (f *freeSpells) Claimed(_ context.Context, id int64) ([]spellbook.FreeSpell, error) {
	return f.claimed[id], nil
}

type env struct {
	hp       hpLedger
	free     *freeSpells
	armour   *wornArmour
	svc      charactersvc.SVC
	reader   *mocks.MockReader
	xp       *mocks.MockXPLedger
	rules    *mocks.MockRules
	owners   *mocks.MockOwners
	sessions *mocks.MockSessions
	tx       *tx
	spells   spellCatalogue
	book     *bookWrites
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWithBook(t, nil)
}

// newEnvWithBook is newEnv with book as the non-weapon catalogue (nil: none).
func newEnvWithBook(t *testing.T, book charactersvc.ProficiencyBook) *env {
	t.Helper()
	e := &env{
		reader: mocks.NewMockReader(t), xp: mocks.NewMockXPLedger(t), rules: newRules(t),
		owners: mocks.NewMockOwners(t), sessions: mocks.NewMockSessions(t), tx: &tx{}, armour: &wornArmour{columns: map[int64]string{}}, hp: hpLedger{},
		spells: wizardCatalogue(), book: &bookWrites{},
		free: &freeSpells{taken: map[int64][]int{}, claimed: map[int64][]spellbook.FreeSpell{}},
	}
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Armour: e.armour, HP: e.hp,
		Proficiencies: book, Spells: e.spells, Book: e.book, FreeSpells: e.free,
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

func TestNew_NilDependency(t *testing.T) {
	full := charactersvc.Deps{
		Reader: mocks.NewMockReader(t), XP: mocks.NewMockXPLedger(t), Rules: newRules(t),
		Owners: mocks.NewMockOwners(t), Tx: &tx{}, Sessions: mocks.NewMockSessions(t), Armour: &wornArmour{}, HP: hpLedger{},
		Spells: wizardCatalogue(), Book: &bookWrites{}, FreeSpells: &freeSpells{},
	}
	without := map[string]func(d *charactersvc.Deps){
		"reader":   func(d *charactersvc.Deps) { d.Reader = nil },
		"xp":       func(d *charactersvc.Deps) { d.XP = nil },
		"rules":    func(d *charactersvc.Deps) { d.Rules = nil },
		"owners":   func(d *charactersvc.Deps) { d.Owners = nil },
		"tx":       func(d *charactersvc.Deps) { d.Tx = nil },
		"sessions": func(d *charactersvc.Deps) { d.Sessions = nil },
		"armour":   func(d *charactersvc.Deps) { d.Armour = nil },
		"hp":       func(d *charactersvc.Deps) { d.HP = nil },
		"spells":   func(d *charactersvc.Deps) { d.Spells = nil },
		"book":     func(d *charactersvc.Deps) { d.Book = nil },
		"free":     func(d *charactersvc.Deps) { d.FreeSpells = nil },
	}
	for name, drop := range without {
		d := full
		drop(&d)
		_, err := charactersvc.New(d)
		require.Error(t, err, name)
	}
}

var owner = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// ADR 0029 steps 7.1 and 7.3: the list is the owner's port answer, each
// character at its realized level (ADR 0030 step 7): Kelsen's XP gives 16,
// level 16 is not taken yet, the list says 15.
func TestList_IsTheOwnersCharactersAtTheirRealizedLevel(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().CharactersOf(mock.Anything, owner).
		Return([]charactersvc.Summary{{ID: 1, Name: "Kelsen"}}, nil).Once()
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(kelsen(), nil).Once()
	e.xp.EXPECT().Awards(mock.Anything, int64(1)).Return(awardsOf(26_582), nil).Once()
	e.rules.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	e.rules.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
	e.rules.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Once()
	got, err := e.svc.List(context.Background(), owner)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.Summary{{
		ID: 1, Name: "Kelsen", Classes: []classes.Entry{classes.SingleEntry("bard", 15, 1_293_418)}, Race: "half_elf",
		Missing: []string{"alignment", "sex"}, MaxHP: 85, CurrentHP: 85,
	}}, got)
}

// The roster card's identity and HP: the race, the alignment and sex codes
// (one that is no code reads as still to choose, ADR 0032 step 8.2), the fields
// to choose, and the current HP derived from the ledger (ADR 0007 step 6, ADR
// 0011 step 1), never above max nor below the floor.
func TestList_CarriesTheIdentityAndTheHP(t *testing.T) {
	for _, tc := range []struct {
		name        string
		alignment   string
		sex         string
		total       int
		wantMissing []string
		wantAlign   string
		wantSex     string
		wantCurrent int
	}{
		{"complete, hurt", "chaotic_good", "male", -30, []string{}, "chaotic_good", "male", 55},
		{"free text alignment reads as missing", "bon et sage", "female", 0, []string{"alignment"}, "", "female", 85},
		{"above max shows max", "chaotic_good", "male", 12, []string{}, "chaotic_good", "male", 85},
		{"below the floor shows the floor", "chaotic_good", "", -400, []string{"sex"}, "chaotic_good", "", -10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			k := kelsen()
			k.Details.Alignment, k.Details.Sex = tc.alignment, tc.sex
			e.hp[1] = tc.total
			e.owners.EXPECT().CharactersOf(mock.Anything, owner).
				Return([]charactersvc.Summary{{ID: 1, Name: "Kelsen"}}, nil).Once()
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(k, nil).Once()
			e.xp.EXPECT().Awards(mock.Anything, int64(1)).Return(awardsOf(26_582), nil).Once()
			e.rules.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
			e.rules.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
			e.rules.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Once()
			got, err := e.svc.List(context.Background(), owner)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, "half_elf", got[0].Race)
			assert.Equal(t, tc.wantAlign, got[0].Alignment)
			assert.Equal(t, tc.wantSex, got[0].Sex)
			assert.Equal(t, tc.wantMissing, got[0].Missing)
			assert.Equal(t, 85, got[0].MaxHP)
			assert.Equal(t, tc.wantCurrent, got[0].CurrentHP)
		})
	}
}

// ADR 0029 step 7.2: setting the current character writes it in one
// transaction, then forgets the owner's cached sessions (after the commit, so
// a lookup cannot cache the old one).
func TestSetCurrent_Step7_2_WritesThenForgetsTheSessions(t *testing.T) {
	e := newEnv(t)
	set := e.owners.EXPECT().SetCurrentCharacter(mock.Anything, owner, int64(4)).Return(nil).Once()
	e.sessions.EXPECT().ForgetUser(owner).Once().NotBefore(set)
	require.NoError(t, e.svc.SetCurrent(context.Background(), owner, 4))
	assert.Equal(t, 1, e.tx.n)
}

// ADR 0029 step 7.1: another user's character (the port matches none) is
// ErrCharacterNotFound, and no session is forgotten.
func TestSetCurrent_Step7_1_AForeignCharacterIsNotFound(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, owner, int64(9)).Return(domain.ErrCharacterNotFound).Once()
	err := e.svc.SetCurrent(context.Background(), owner, 9)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

// GetCharacter derives the character's block from what the reader returns
// and what the XP ledger's entries add (ADR 0030 steps 3 to 8), and passes a
// reader's error through.
func TestGetCharacter(t *testing.T) {
	t.Run("derived from the reader's character and the ledger", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, nil).Once()
		expectKelsenRules(e.rules, 1_293_418, 15)

		got, err := e.svc.GetCharacter(context.Background(), 3)
		require.NoError(t, err)
		want := kelsen()
		want.Derived = kelsenDerived()
		// ADR 0037 step 15: a bard's 230 points left to spend; no catalogue, no pool.
		want.Derived.LevelUp.ToSpend = skills.LevelGrants{SkillPoints: 230}
		want.Details.Languages, want.Warnings = []string{}, []charactersvc.Warning{}
		assert.Equal(t, want, got)
	})
	t.Run("not found passes sentinel through", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(nil, domain.ErrCharacterNotFound).Once()
		got, err := e.svc.GetCharacter(context.Background(), 3)
		assert.True(t, errors.Is(err, domain.ErrCharacterNotFound))
		assert.Nil(t, got)
	})
	t.Run("a ledger error is the read's", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(kelsen(), nil).Once()
		boom := errors.New("boom")
		e.xp.EXPECT().Awards(mock.Anything, int64(3)).Return(nil, boom).Once()
		_, err := e.svc.GetCharacter(context.Background(), 3)
		require.ErrorIs(t, err, boom)
	})
}

// Score reads each ability's own field of the sheet (distinct values, so a
// crossed field fails) and refuses an unknown key.
func TestCharacter_Score(t *testing.T) {
	c := &charactersvc.Character{Str: 11, Dex: 12, Con: 13, Int: 14, Wis: 15, Cha: 16}
	want := map[charactersvc.Ability]int{"str": 11, "dex": 12, "con": 13, "int": 14, "wis": 15, "cha": 16}
	require.Len(t, charactersvc.Abilities, len(want))
	for _, a := range charactersvc.Abilities {
		got, ok := c.Score(a)
		assert.True(t, ok, a)
		assert.Equal(t, want[a], got, a)
	}
	_, ok := c.Score("luck")
	assert.False(t, ok)
}

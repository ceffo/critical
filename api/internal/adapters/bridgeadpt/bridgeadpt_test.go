package bridgeadpt_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/bridgeadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

var errBoom = errors.New("boom")

type stubRounds struct {
	n   int
	err error
}

func (s stubRounds) GetRoundNumber(context.Context, int64) (int, error) { return s.n, s.err }

func TestRoundReaderReadsTheRoundNumber(t *testing.T) {
	got, err := bridgeadpt.RoundReader{DAL: stubRounds{n: 7}}.GetRound(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 7, got)

	_, err = bridgeadpt.RoundReader{DAL: stubRounds{err: errBoom}}.GetRound(context.Background(), 1)
	assert.ErrorIs(t, err, errBoom)
}

type stubItems struct {
	itemsvc.DAL
	pieces []itemsvc.Item
	err    error
}

func (s stubItems) List(context.Context, int64) ([]itemsvc.Item, error) { return s.pieces, s.err }

func TestWornArmourReadsTheWornBodyArmoursColumn(t *testing.T) {
	pieces := []itemsvc.Item{
		{Category: itemsvc.CategoryArmour, State: itemsvc.StateCarried, SkillColumn: "studded"},
		{Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: "leather"},
	}
	got, err := bridgeadpt.WornArmour{DAL: stubItems{pieces: pieces}}.WornSkillColumn(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "leather", got)

	got, err = bridgeadpt.WornArmour{DAL: stubItems{}}.WornSkillColumn(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, got, "nothing worn is no column")

	_, err = bridgeadpt.WornArmour{DAL: stubItems{err: errBoom}}.WornSkillColumn(context.Background(), 1)
	assert.ErrorIs(t, err, errBoom)
}

type stubForms struct {
	form   combatsvc.AttackForm
	stored bool
}

func (s stubForms) Get(context.Context, int64) (combatsvc.AttackForm, bool, error) {
	return s.form, s.stored, nil
}
func (s stubForms) Put(context.Context, int64, combatsvc.AttackForm) error { return nil }

type stubWeapons struct {
	combatsvc.Weapons
	list []combatsvc.Weapon
}

func (s stubWeapons) List(context.Context, int64) ([]combatsvc.Weapon, error) { return s.list, nil }

func TestWeaponInHandTranslatesTheWieldedWeapon(t *testing.T) {
	list := []combatsvc.Weapon{
		{ID: "dagger", Name: localized.Of("Dagger")},
		{
			ID: "sword", Name: localized.Of("Sword"), Wielded: true, Kind: combatsvc.WeaponKind("melee"),
			Speed: 5, MagicToHit: 2, MagicDamage: 3,
		},
	}
	w := bridgeadpt.WeaponInHand{Forms: stubForms{}, Weapons: stubWeapons{list: list}}

	got, ok, err := w.WeaponInHand(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, buffsvc.HeldWeapon{
		ItemID: "sword", Name: "Sword", Kind: buffsvc.WeaponKind("melee"), Speed: 5, MagicToHit: 2, MagicDamage: 3,
	}, got)
}

func TestWeaponInHandIsFalseWithNothingWielded(t *testing.T) {
	w := bridgeadpt.WeaponInHand{
		Forms:   stubForms{},
		Weapons: stubWeapons{list: []combatsvc.Weapon{{ID: "dagger"}}},
	}
	got, ok, err := w.WeaponInHand(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, buffsvc.HeldWeapon{}, got)
}

type stubLog struct {
	actionlogsvc.SVC
	entries []actionlog.Entry
	err     error
	limit   int
	kind    actionlog.Kind
}

func (s *stubLog) ActiveEntries(_ context.Context, _ int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error) {
	s.kind, s.limit = kind, limit
	return s.entries, s.err
}

type stubClaims struct{ claimed []spellbook.FreeSpell }

func (s stubClaims) Claimed(context.Context, int64) ([]spellbook.FreeSpell, error) {
	return s.claimed, nil
}

func levelUp(id int64, payload string) actionlog.Entry {
	return actionlog.Entry{ID: id, Kind: actionlog.KindLevelUp, Payload: json.RawMessage(payload)}
}

func TestFreeSpellsLevelsTakenDecodesEachActiveLevelUp(t *testing.T) {
	log := &stubLog{entries: []actionlog.Entry{
		levelUp(1, `{"class":"a","level":2}`),
		levelUp(2, `{"class":"b","level":1}`),
		levelUp(3, `{"class":"a","level":3}`),
	}}
	f := bridgeadpt.FreeSpells{Log: log, Book: stubClaims{}}

	got, err := f.LevelsTaken(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.TakenLevel{{Class: "a", Level: 2}, {Class: "b", Level: 1}, {Class: "a", Level: 3}}, got)
	assert.Equal(t, actionlog.KindLevelUp, log.kind)
	assert.Equal(t, classes.MaxMultiClasses*ruletablesvc.MaxLevel, log.limit)
}

func TestFreeSpellsLevelsTakenRefusesABrokenEntry(t *testing.T) {
	for name, payload := range map[string]string{"not json": `nope`, "level below 1": `{"class":"a","level":0}`} {
		t.Run(name, func(t *testing.T) {
			f := bridgeadpt.FreeSpells{Log: &stubLog{entries: []actionlog.Entry{levelUp(9, payload)}}}
			_, err := f.LevelsTaken(context.Background(), 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "level_up entry 9")
		})
	}
}

func TestFreeSpellsLevelsTakenPassesTheLogsError(t *testing.T) {
	_, err := bridgeadpt.FreeSpells{Log: &stubLog{err: errBoom}}.LevelsTaken(context.Background(), 1)
	assert.ErrorIs(t, err, errBoom)
}

func TestFreeSpellsClaimedReadsTheBook(t *testing.T) {
	claims := []spellbook.FreeSpell{{}}
	got, err := bridgeadpt.FreeSpells{Book: stubClaims{claimed: claims}}.Claimed(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, claims, got)
}

func TestProficiencyIDsKnowsNothingOfAnEmptyBook(t *testing.T) {
	assert.False(t, bridgeadpt.ProficiencyIDs{Book: &ruletablesvc.Proficiencies{}}.Has("swimming"))
}

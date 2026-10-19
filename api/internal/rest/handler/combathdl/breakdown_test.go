package combathdl_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/rest/handler/combathdl"
)

const charID = int64(1)

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// The attack's answer and its log entry's payload are one struct
// (combatsvc.AttackBreakdown): the bytes the endpoint sends are the bytes it
// logged, plus the entry's id as the last key. Checked on the roll (an
// attack with ammunition, so every optional part is present) and on a
// target AC correction (the amended payload).
func TestAttack_AnswerIsTheLoggedPayload(t *testing.T) {
	artifact, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	rules, err := ruletablesadpt.Parse(artifact.Catalogue.Format, artifact.RuleTables, artifact.Equipment, artifact.Languages)
	require.NoError(t, err)

	chars, buffs, dice := mocks.NewMockCharacters(t), mocks.NewMockBuffs(t), mocks.NewMockDice(t)
	weapons, items, crit, log := mocks.NewMockWeapons(t), mocks.NewMockItems(t), mocks.NewMockCritStore(t), mocks.NewMockLog(t)
	svc, err := combatsvc.NewWithConfig(combatsvc.DefaultConfig(), combatsvc.Deps{
		Chars: chars, Buffs: buffs, Dice: dice, Weapons: weapons, Crit: crit,
		Items: items, Forms: mocks.NewMockFormStore(t), Rules: rules, Log: log, Tx: noTx{},
	})
	require.NoError(t, err)

	shortbow := combatsvc.Weapon{
		Wielded: true, ID: "shortbow", Name: localized.Of("Short Bow +2"), BaseDamage: "1d6", DamageLarge: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow",
	}
	arrows := itemsvc.Item{
		ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		MagicBonus: 3, Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Count: 1,
	}
	chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{
		ID: charID, Str: 12, Dex: 17, Class: "bard", Race: "half_elf",
		Derived: charactersvc.Derived{RealizedLevel: 15, THAC0: 13},
	}, nil)
	weapons.EXPECT().Get(mock.Anything, charID, "shortbow").Return(shortbow, nil)
	buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).
		Return(buffsvc.BuffModifiers{SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12}, nil)
	items.EXPECT().Use(mock.Anything, charID, "arrows_plus3").Return(arrows, nil).Once()
	crit.EXPECT().List(mock.Anything).Return(nil, nil).Maybe()
	dice.EXPECT().RollD20().Return(14).Once()
	dice.EXPECT().RollD100().Return(30).Once()
	dice.EXPECT().RollD100().Return(77).Once()
	dice.EXPECT().RollDie(6).Return(4).Once()

	const entryID = int64(42)
	entry := actionlog.Entry{ID: entryID, CharacterID: charID, Kind: actionlog.KindAttack}
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			entry.Payload = e.Payload
			return entryID, nil
		}).Once()
	log.EXPECT().AmendLatest(mock.Anything, charID, entryID, actionlog.KindAttack, mock.Anything).RunAndReturn(
		func(_ context.Context, _, _ int64, _ actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
			a, err := fn(entry)
			if err != nil {
				return nil, err
			}
			entry.Corrections = append(entry.Corrections, actionlog.Correction{
				ActionID: entryID, Kind: actionlog.CorrectionAmend, Payload: a.Payload,
			})
			return &entry, nil
		}).Once()

	r := chi.NewRouter()
	combathdl.RegisterRoutes(r, func(*http.Request) int64 { return charID }, svc)
	post := func(path, body string) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return bytes.TrimSuffix(rec.Body.Bytes(), []byte("\n"))
	}
	// withEntryID is payload with the entry id appended as its last key.
	withEntryID := func(payload json.RawMessage) string {
		require.True(t, bytes.HasSuffix(payload, []byte("}")))
		return string(payload[:len(payload)-1]) + `,"logEntryId":` + strconv.FormatInt(entryID, 10) + "}"
	}

	rolled := post("/api/roll/attack", `{"weapon_id":"shortbow","ammo_item_id":"arrows_plus3","target_ac":2}`)
	require.NotEmpty(t, entry.Payload)
	assert.Equal(t, withEntryID(entry.Payload), string(rolled))
	assert.NotContains(t, string(entry.Payload), "logEntryId", "the payload carries no entry id")
	for _, key := range []string{`"ammo":`, `"rolled":`, `"critRow":`, `"critPossible":`, `"damageRolls":`} {
		assert.Contains(t, string(rolled), key)
	}

	corrected := post("/api/roll/attack/42/adjust", `{"targetAc":7}`)
	require.Len(t, entry.Corrections, 1)
	assert.Equal(t, withEntryID(entry.EffectivePayload()), string(corrected))
	assert.Contains(t, string(corrected), `"rolledTargetAc":2`)
}

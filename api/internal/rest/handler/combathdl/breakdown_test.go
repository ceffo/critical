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

const (
	charID  = int64(1)
	entryID = int64(42)
)

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// wire is the real combat service on mocked ports behind the combat routes,
// with a log that keeps the one entry it records (id entryID) and applies
// its amend.
type wire struct {
	chars   *mocks.MockCharacters
	buffs   *mocks.MockBuffs
	dice    *mocks.MockDice
	weapons *mocks.MockWeapons
	items   *mocks.MockItems
	router  http.Handler
	entry   actionlog.Entry
}

// newWire builds the wire; a non-nil logged is the entry's payload as an
// older release logged it (nothing is recorded then).
func newWire(t *testing.T, kind actionlog.Kind, logged json.RawMessage) *wire {
	t.Helper()
	artifact, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	rules, err := ruletablesadpt.Parse(artifact.Catalogue.Format, artifact.RuleTables, artifact.Equipment, artifact.Languages)
	require.NoError(t, err)

	w := &wire{
		chars: mocks.NewMockCharacters(t), buffs: mocks.NewMockBuffs(t), dice: mocks.NewMockDice(t),
		weapons: mocks.NewMockWeapons(t), items: mocks.NewMockItems(t),
		entry: actionlog.Entry{ID: entryID, CharacterID: charID, Kind: kind, Payload: logged},
	}
	crit, log := mocks.NewMockCritStore(t), mocks.NewMockLog(t)
	crit.EXPECT().List(mock.Anything).Return(nil, nil).Maybe()
	if logged == nil {
		log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, e actionlog.NewEntry) (int64, error) {
				w.entry.Payload = e.Payload
				return entryID, nil
			}).Once()
	}
	log.EXPECT().AmendLatest(mock.Anything, charID, entryID, kind, mock.Anything).RunAndReturn(
		func(_ context.Context, _, _ int64, _ actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
			a, err := fn(w.entry)
			if err != nil {
				return nil, err
			}
			w.entry.Corrections = append(w.entry.Corrections, actionlog.Correction{
				ActionID: entryID, Kind: actionlog.CorrectionAmend, Payload: a.Payload,
			})
			return &w.entry, nil
		}).Once()

	svc, err := combatsvc.NewWithConfig(combatsvc.DefaultConfig(), combatsvc.Deps{
		Chars: w.chars, Buffs: w.buffs, Dice: w.dice, Weapons: w.weapons, Crit: crit,
		Items: w.items, Forms: mocks.NewMockFormStore(t), Rules: rules, Log: log, Tx: noTx{},
	})
	require.NoError(t, err)
	r := chi.NewRouter()
	combathdl.RegisterRoutes(r, func(*http.Request) int64 { return charID }, svc)
	w.router = r

	w.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{
		ID: charID, Str: 12, Dex: 17, Class: "bard", Race: "dwarf",
		Derived: charactersvc.Derived{
			RealizedLevel: 15, THAC0: 13,
			Saves:         charactersvc.Saves{ParaPoison: 11, RodStaffWand: 13, PetriPoly: 12, Breath: 15, Spell: 14},
			RaceSaveBonus: charactersvc.Saves{Spell: 4},
		},
	}, nil).Maybe()
	return w
}

// post sends body to path and returns the answer without the encoder's
// final newline.
func (w *wire) post(t *testing.T, path, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return string(bytes.TrimSuffix(rec.Body.Bytes(), []byte("\n")))
}

// withKeys is payload with the entry id, then tail, appended as its last
// keys.
func withKeys(t *testing.T, payload json.RawMessage, tail string) string {
	t.Helper()
	require.True(t, bytes.HasSuffix(payload, []byte("}")))
	assert.NotContains(t, string(payload), "logEntryId", "the payload carries no entry id")
	return string(payload[:len(payload)-1]) + `,"logEntryId":` + strconv.FormatInt(entryID, 10) + tail + "}"
}

// The attack's answer and its log entry's payload are one struct
// (combatsvc.AttackBreakdown): the bytes the endpoint sends are the bytes it
// logged, plus the entry's id as the last key. Checked on the roll (an
// attack with ammunition, so every optional part is present) and on a
// target AC correction (the amended payload).
func TestAttack_AnswerIsTheLoggedPayload(t *testing.T) {
	w := newWire(t, actionlog.KindAttack, nil)
	w.weapons.EXPECT().Get(mock.Anything, charID, "shortbow").Return(combatsvc.Weapon{
		Wielded: true, ID: "shortbow", Name: localized.Of("Short Bow +2"), BaseDamage: "1d6", DamageLarge: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow",
	}, nil)
	w.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).
		Return(buffsvc.BuffModifiers{SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12}, nil)
	w.items.EXPECT().Use(mock.Anything, charID, "arrows_plus3").Return(itemsvc.Item{
		ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		MagicBonus: 3, Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Count: 1,
	}, nil).Once()
	w.dice.EXPECT().RollD20().Return(14).Once()
	w.dice.EXPECT().RollD100().Return(30).Once()
	w.dice.EXPECT().RollD100().Return(77).Once()
	w.dice.EXPECT().RollDie(6).Return(4).Once()

	rolled := w.post(t, "/api/roll/attack", `{"weapon_id":"shortbow","ammo_item_id":"arrows_plus3","target_ac":2}`)
	require.NotEmpty(t, w.entry.Payload)
	assert.Equal(t, withKeys(t, w.entry.Payload, ""), rolled)
	for _, key := range []string{`"ammo":`, `"rolled":`, `"critRow":`, `"critPossible":`, `"damageRolls":`} {
		assert.Contains(t, rolled, key)
	}

	corrected := w.post(t, "/api/roll/attack/42/adjust", `{"targetAc":7}`)
	require.Len(t, w.entry.Corrections, 1)
	assert.Equal(t, withKeys(t, w.entry.EffectivePayload(), ""), corrected)
	assert.Contains(t, corrected, `"rolledTargetAc":2`)
}

// The save's answer and its log entry's payload are one struct
// (combatsvc.SaveBreakdown): the bytes the endpoint sends are the bytes it
// logged, plus the entry's id, then hit, hitMargin and baseThac0 for the
// previous release's web (kept for one release). Checked on the roll (a race
// modifier and a buff) and on an adjustment (the amended payload).
func TestSave_AnswerIsTheLoggedPayload(t *testing.T) {
	w := newWire(t, actionlog.KindSave, nil)
	mods := buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, SaveBonus: 1,
		SaveBonusFrom: []buffsvc.Contribution{{Buff: "Luck", Value: 1}},
	}
	w.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponNone).Return(mods, nil).Once()
	w.dice.EXPECT().RollD20().Return(8).Once()

	// 8 + 4 (dwarf) + 1 (Luck) = 13 against 14: failed by 1.
	rolled := w.post(t, "/api/roll/save", `{"category":"spell"}`)
	require.NotEmpty(t, w.entry.Payload)
	assert.Equal(t, withKeys(t, w.entry.Payload, `,"hit":false,"hitMargin":-1,"baseThac0":14`), rolled)
	assert.Contains(t, rolled, `"rollTotal":13`)

	// +2 moves the target to 12: passed by 1.
	adjusted := w.post(t, "/api/roll/save/42/adjust", `{"adjustment":2,"reason":"Gaia fire"}`)
	require.Len(t, w.entry.Corrections, 1)
	assert.Equal(t, withKeys(t, w.entry.EffectivePayload(), `,"hit":true,"hitMargin":1,"baseThac0":14`), adjusted)
	assert.Contains(t, adjusted, `"previousPassed":false`)
}

// An attack logged before critPossible was recorded (its "source" labels
// from before the UI named modifiers included), then corrected: the answer
// is the amended payload, which gains critPossible, plus the entry id. No
// view serves attack entries, so the amended payload is what the journal
// shows.
func TestAttack_CorrectedLegacyEntry_AnswerIsTheAmendedPayload(t *testing.T) {
	w := newWire(t, actionlog.KindAttack, json.RawMessage(`{"d20Roll":14,"d100CritRoll":30,"d100MultiplierRoll":77,"isCrit":false,`+
		`"critMultiplier":1,"critPlaceholder":false,"critFrom":0,"critTo":0,"baseThac0":13,`+
		`"modifiers":[{"kind":"dexterity","detail":"17","value":2,"source":"DEX 17"},{"kind":"weapon_magic","detail":"Short Bow +2","value":2}],`+
		`"effectiveThac0":9,"targetACNeeded":-5,"hitsAc":-5,"critsFromAc":25,"hit":true,"hitMargin":7,`+
		`"baseDamage":4,"damageDice":"1d6","spellsBuff":1,"weaponBuff":2,"multipliers":1,"diceMultiplier":1,`+
		`"strengthBonus":0,"bonusDamage":0,"damageTerms":[],"totalDamage":6,"weaponId":"shortbow","weaponName":"Short Bow +2",`+
		`"targetAc":2,"targetSize":"sm","acKnown":true,`+
		`"rolled":{"dice":"1d6","total":4,"spellsBuff":1,"weaponBuff":2,"buffMultipliers":1,"strengthBonus":0,"bonusDamage":0,"terms":[]},`+
		`"critRow":{"from":1,"to":100,"multiplier":2,"placeholder":true}}`))

	corrected := w.post(t, "/api/roll/attack/42/adjust", `{"targetAc":7}`)
	require.Len(t, w.entry.Corrections, 1)
	amended := w.entry.EffectivePayload()
	assert.Equal(t, withKeys(t, amended, ""), corrected)
	assert.Contains(t, string(amended), `"critPossible":`, "the amend records critPossible")
	assert.Contains(t, corrected, `"rolledTargetAc":2`)
}

// A save logged before the adjustment moved the target (no rollTotal or
// adjustedTarget, an English "label"), then adjusted: the answer is the
// amended payload as SavePayloadView serves it, plus the entry id and the
// previous release's three keys.
func TestSave_AdjustedLegacyEntry_AnswerIsTheViewedPayload(t *testing.T) {
	legacy := json.RawMessage(`{"category":"spell","label":"Spells","d20Roll":12,` +
		`"modifiers":[{"kind":"buff","detail":"Luck","value":1,"source":"Luck"}],"target":14,"rolledPassed":false,` +
		`"adjustment":0,"total":13,"passed":false,"margin":-1}`)
	w := newWire(t, actionlog.KindSave, legacy)
	viewed, err := combatsvc.SavePayloadView(legacy)
	require.NoError(t, err)
	assert.Contains(t, string(viewed), `"rollTotal":13`, "the view fills the old entry")

	// +2 moves the target to 12: 12 + 1 = 13 passes by 1.
	adjusted := w.post(t, "/api/roll/save/42/adjust", `{"adjustment":2,"reason":"Gaia fire"}`)
	require.Len(t, w.entry.Corrections, 1)
	served, err := combatsvc.SavePayloadView(w.entry.EffectivePayload())
	require.NoError(t, err)
	assert.Equal(t, withKeys(t, served, `,"hit":true,"hitMargin":1,"baseThac0":14`), adjusted)
	assert.Contains(t, adjusted, `"rollTotal":13,"adjustedTarget":12`)
}

package composition_test

// ADR 0018 step 2: undoing an entry writes every snapshot it carries back
// through the codec of its state-key family. The root registers every
// service's module on the log, so the families come from the module list:
// each declared family has a case below, and every service the root holds is
// on the list.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/postgres/snapshotfixtures"
)

// reencoded is a stored state as the migration data step re-encodes it from its old encoding:
// the envelopes the data step's own test checks (storage/postgres snapshotfixtures), so a
// restorer here reads exactly what the step writes.
func reencoded(name string) string { return snapshotfixtures.Envelope(name) }

// undoCase is the undo of one entry whose one snapshot is key's, before
// stored as the migration data step writes it, and the storage call the
// owning service's codec makes.
type undoCase struct {
	name   string
	key    string
	before string
	expect func(h *harness)
}

func undoCases() []undoCase {
	return []undoCase{
		{"active buffs", "buffs", reencoded("no buff"), func(h *harness) {
			h.Buff.EXPECT().SaveActiveBuffs(mock.Anything, characterID, []buffsvc.BuffEffect{}).Return(nil).Once()
		}},
		{"round number", "round", reencoded("round (buffsvc)"), func(h *harness) {
			h.Buff.EXPECT().SaveRoundNumber(mock.Anything, characterID, 3).Return(nil).Once()
		}},
		{"saved buff templates", "saved_buffs", reencoded("saved templates (buffsvc)"), func(h *harness) {
			h.Buff.EXPECT().ReplaceSavedBuffs(mock.Anything, characterID, mock.Anything).Return(nil).Once()
		}},
		{"initiative cleared", "initiative", reencoded("no initiative (buffsvc): null is absent"), func(h *harness) {
			h.Buff.EXPECT().SaveInitiative(mock.Anything, characterID, (*buffsvc.Initiative)(nil)).Return(nil).Once()
		}},
		{"spell slots", "spell_slots", reencoded("spell slots (prepsvc)"), func(h *harness) {
			h.Spell.EXPECT().SaveSlots(mock.Anything, characterID, mock.MatchedBy(func(s []spellsvc.SpellSlot) bool {
				return len(s) == 1 && s[0].SpellID == "alarm"
			})).Return(nil).Once()
		}},
		{"an item's count (family items), the entry's character's", "items:wand_lightning", reencoded("item count of the equipment model (itemsvc)"), func(h *harness) {
			h.Items.EXPECT().SetCount(mock.Anything, characterID, "wand_lightning", 4).Return(nil).Once()
		}},
		{"a weapon added by hand before the equipment model (family weapons): the piece", "weapons:dagger2", reencoded("weapon added before the equipment model (weaponsvc): absent"), func(h *harness) {
			h.Items.EXPECT().Delete(mock.Anything, characterID, "dagger2").Return(nil).Once()
		}},
		{"a level-up roll's realized level", "realized_level", reencoded("realized level (levelupsvc)"), func(h *harness) {
			h.Character.EXPECT().SetRealizedLevel(mock.Anything, characterID, 15, 85).Return(nil).Once()
		}},
		{"a learned spell (family spellbook) taken out", "spellbook:sleep", reencoded("spellbook (learnsvc): the spell goes to the envelope"), func(h *harness) {
			h.Spellbook.EXPECT().RemoveEntry(mock.Anything, characterID, "sleep").Return(nil).Once()
			h.Spellbook.EXPECT().DeleteLearnFailure(mock.Anything, characterID, "sleep").Return(nil).Once()
		}},
		{"a spell back in the spellbook (family spellbook)", "spellbook:sleep", reencoded("spellbook, a known spell"), func(h *harness) {
			h.Spellbook.EXPECT().AddEntry(mock.Anything, characterID, "sleep").Return(nil).Once()
			h.Spellbook.EXPECT().DeleteLearnFailure(mock.Anything, characterID, "sleep").Return(nil).Once()
		}},
		{"an older learn failure put back (family spellbook)", "spellbook:light", reencoded("spellbook, a failure"), func(h *harness) {
			h.Spellbook.EXPECT().RemoveEntry(mock.Anything, characterID, "light").Return(nil).Once()
			h.Spellbook.EXPECT().SetLearnFailure(mock.Anything, characterID, "light", 4).Return(nil).Once()
		}},
		{"a thief's skill armour", "skill_armor", reencoded("skill armour (charactersvc)"), func(h *harness) {
			h.Character.EXPECT().RestoreSkillArmor(mock.Anything, characterID, "none").Return(nil).Once()
		}},
		{"an XP entry added by hand (family xp)", "xp:12", reencoded("XP entry added (xpsvc): absent, with its zeroed fields"), func(h *harness) {
			h.XP.EXPECT().Delete(mock.Anything, characterID, int64(12)).Return(nil).Once()
		}},
		{"a thief's skill points", "skill_points", reencoded("skill points (skillsvc)"), func(h *harness) {
			h.Character.EXPECT().SetSkillPoints(mock.Anything, characterID, map[string]int{"climb_walls": 80}).Return(nil).Once()
		}},
		// Recorded with the codec's envelope from the start: no older encoding to re-encode.
		{"the proficiency picks (ADR 0037 step 13)", "proficiencies", `{"state":{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"dagger","slots":1}]}}`, func(h *harness) {
			h.Character.EXPECT().SetProficiencyPicks(mock.Anything, characterID, charactersvc.Picks{
				Nonweapon: []charactersvc.Pick{{ID: "tumbling", Slots: 1}}, Weapon: []charactersvc.WeaponPick{{Type: "dagger", Slots: 1}},
			}).Return(nil).Once()
		}},
		{"the AC override cleared", "ac_override", reencoded("AC override (acsvc)"), func(h *harness) {
			h.Character.EXPECT().SetACOverride(mock.Anything, characterID, (*acsvc.Override)(nil)).Return(nil).Once()
		}},
	}
}

func TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage(t *testing.T) {
	const entryID = int64(40)
	for _, tc := range undoCases() {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.expectStartup(fixtureSeeds)
			bs, err := h.build(t, fixtureConfig())
			require.NoError(t, err)

			entry := &actionlog.Entry{
				ID: entryID, TransactionID: entryID, CharacterID: characterID, Kind: actionlog.KindBuffChange,
				Snapshots: []actionlog.Snapshot{{Key: tc.key, Before: json.RawMessage(tc.before), After: json.RawMessage(`null`)}},
			}
			h.Log.EXPECT().LatestActiveID(mock.Anything, characterID).Return(entryID, true, nil).Once()
			h.Log.EXPECT().GetEntry(mock.Anything, characterID, entryID).Return(entry, nil).Once()
			h.Log.EXPECT().ListTransaction(mock.Anything, characterID, entryID).Return([]actionlog.Entry{*entry}, nil).Once()
			tc.expect(h)
			h.Log.EXPECT().InsertCorrection(mock.Anything, actionlog.Correction{ActionID: entryID, Kind: actionlog.CorrectionVoid}).Return(nil).Once()
			h.Log.EXPECT().GetEntry(mock.Anything, characterID, entryID).Return(entry, nil).Once()

			_, err = bs.Log.Undo(context.Background(), characterID, entryID)
			require.NoError(t, err)
		})
	}
}

// The families the undo cases cover are the families the modules declare:
// a service that starts snapshotting a family fails here until its undo has
// a case, and a case for a family no module declares fails too.
func TestModules_EveryDeclaredFamilyHasAnUndoCase(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)

	declared := map[string]string{}
	for _, svc := range composition.Services(bs) {
		m := svc.Module()
		for _, c := range m.Codecs {
			declared[c.Family()] = m.Name
		}
	}
	covered := map[string]string{}
	for _, tc := range undoCases() {
		covered[actionlog.Family(tc.key)] = declared[actionlog.Family(tc.key)]
	}
	assert.Equal(t, declared, covered)
}

// Every service the root holds is on its one list of services: its module is
// registered and Close reaches it.
func TestBuild_EveryServiceIsOnTheModuleList(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)

	listed := map[domain.Service]bool{}
	for _, svc := range composition.Services(bs) {
		require.True(t, reflect.ValueOf(svc).Comparable(), "%s is not comparable", svc.Module().Name)
		assert.False(t, listed[svc], "%s listed twice", svc.Module().Name)
		listed[svc] = true
	}
	service := reflect.TypeFor[domain.Service]()
	v := reflect.ValueOf(bs).Elem()
	held := 0
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if !f.IsExported() || !f.Type.Implements(service) {
			continue
		}
		held++
		svc, ok := v.Field(i).Interface().(domain.Service)
		require.True(t, ok && svc != nil, "field %s is not built", f.Name)
		assert.True(t, listed[svc], "field %s is not on the module list", f.Name)
	}
	assert.Equal(t, held, len(listed), "the list holds a service no field serves")
}

// Two modules claiming one state-key family refuse the start, naming both: a
// new module claiming a family of the built root is refused by the loop Build
// ends with.
func TestBuild_TwoModulesClaimingOneFamilyRefuseTheStart(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)

	err = composition.RegisterModules(bs.Log, []domain.Service{claimer{"buffs"}})
	require.ErrorIs(t, err, actionlogsvc.ErrClaimed)
	assert.ErrorContains(t, err, "buffsvc")
	assert.ErrorContains(t, err, "claimer")
}

// claimer is a service whose module claims family.
type claimer struct{ family string }

func (claimer) Health(context.Context) error { return nil }
func (claimer) Close(context.Context) error  { return nil }
func (c claimer) Module() domain.Module {
	return domain.Module{Name: "claimer", Codecs: []actionlog.Codec{
		actionlog.NewSnapshotter[int](c.family).Restorer(func(context.Context, int64, string, int) error { return nil }, nil),
	}}
}

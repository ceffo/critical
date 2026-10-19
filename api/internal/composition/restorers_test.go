package composition_test

// ADR 0018 step 2: undoing an entry writes every snapshot it carries back
// through the restorer of its state key. The root registers each service's
// restorers on the log, so a state key left unregistered would fail the undo
// of its entries ("no restorer registered").

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/postgres/snapshotfixtures"
)

// reencoded is a stored state as the migration data step re-encodes it from its old encoding:
// the envelopes the data step's own test checks (storage/postgres snapshotfixtures), so a
// restorer here reads exactly what the step writes.
func reencoded(name string) string { return snapshotfixtures.Envelope(name) }

func TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage(t *testing.T) {
	const entryID = int64(40)
	tests := []struct {
		name   string
		key    string
		before string
		expect func(h *harness)
	}{
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
	}
	for _, tc := range tests {
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

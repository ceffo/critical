package composition_test

// ADR 0018 steps 1 and 2: every table-state mutation is one transaction, so
// one undo walks the whole state back. Exercised on the real SQLite log with
// every restorer wired.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// stackTables hold the state the action stack restores (ADR 0010 Context,
// ADR 0018 scope). Tables outside the stack (character details, crit table,
// dice favourites, the attack form) are not compared.
var stackTables = []string{"active_buffs", "spell_slots", "round_state", "items", "weapons", "saved_buffs", "xp_entries"}

// tableState is every row of the stack's tables, current HP and the log as
// it reads (each non-voided entry with its effective payload): what one undo
// must give back.
type tableState struct {
	Tables map[string][]string
	HP     int
	Log    []string
}

func tableStateOf(t *testing.T, bs *composition.BackingServices) tableState {
	t.Helper()
	ctx := context.Background()
	out := tableState{Tables: map[string][]string{}}
	for _, table := range stackTables {
		out.Tables[table] = dumpRows(t, bs.DB.DB, table)
	}
	hp, err := bs.HP.GetState(ctx, bs.CharacterID)
	require.NoError(t, err)
	out.HP = hp.CurrentHp
	entries, err := bs.Log.List(ctx, bs.CharacterID, 500)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Status() != actionlog.StatusVoided {
			out.Log = append(out.Log, fmt.Sprintf("#%d %s %s", e.ID, e.Kind, e.EffectivePayload()))
		}
	}
	return out
}

func dumpRows(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT * FROM ` + table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		raw, err := json.Marshal(vals)
		require.NoError(t, err)
		out = append(out, string(raw))
	}
	require.NoError(t, rows.Err())
	// A restored row (a deleted item put back) may land at another rowid:
	// compare contents, not physical order.
	slices.Sort(out)
	return out
}

// mutation is one state-changing request. setup brings the table to where
// the request is possible; it is not undone.
type mutation struct {
	name  string
	setup func(ctx context.Context, bs *composition.BackingServices) error
	run   func(ctx context.Context, bs *composition.BackingServices) error
}

func attackOn(ac *int) func(ctx context.Context, bs *composition.BackingServices) (*combatsvc.RollResult, error) {
	return func(ctx context.Context, bs *composition.BackingServices) (*combatsvc.RollResult, error) {
		return bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: ac})
	}
}

// stackMutations is every request that writes table state through the action
// stack (the epic's audit list). A new state-changing service belongs here.
func stackMutations() []mutation {
	var lastAttack, lastSave int64
	var savedID, xpID int64
	award := func(ctx context.Context, bs *composition.BackingServices) error {
		l, err := bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 1205, Bonus: true, Reason: "troll"})
		if err == nil {
			xpID = l.Entries[0].ID
		}
		return err
	}
	var buffID string
	activate := func(ctx context.Context, bs *composition.BackingServices) error {
		list, err := bs.Buffs.Activate(ctx, bs.CharacterID, buffsvc.BuffEffect{Name: "Stoneskin", Charges: 3, Duration: 4})
		if err == nil {
			buffID = list[len(list)-1].ID
		}
		return err
	}
	shot := func(ac *int) func(ctx context.Context, bs *composition.BackingServices) error {
		return func(ctx context.Context, bs *composition.BackingServices) error {
			res, err := attackOn(ac)(ctx, bs)
			if err == nil {
				lastAttack = res.LogEntryID
			}
			return err
		}
	}
	advance := func(ctx context.Context, bs *composition.BackingServices) error {
		_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, bs.CharacterID)
		return err
	}
	prepareAndCast := func(ctx context.Context, bs *composition.BackingServices) error {
		if _, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"stoneskin"}); err != nil {
			return err
		}
		_, err := bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{})
		return err
	}
	return []mutation{
		{name: "HP change", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -7, "troll claw")
			return err
		}},
		{name: "exposure on the Bard", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Cast.Expose(ctx, bs.CharacterID, castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 15, OnSelf: true})
			return err
		}},
		{name: "spell preparation", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"stoneskin"})
			return err
		}},
		{name: "cast", setup: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"stoneskin"})
			return err
		}, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{})
			return err
		}},
		{name: "receive a spell", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Cast.Receive(ctx, bs.CharacterID, castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9})
			return err
		}},
		{name: "long rest", setup: func(ctx context.Context, bs *composition.BackingServices) error {
			if err := prepareAndCast(ctx, bs); err != nil {
				return err
			}
			if _, err := bs.Items.UseItem(ctx, bs.CharacterID, "wand_lightning"); err != nil {
				return err
			}
			_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -9, "ogre")
			return err
		}, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Rest.LongRest(ctx, bs.CharacterID)
			return err
		}},
		{name: "buff activate", run: activate},
		{name: "buff edit", setup: activate, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.Edit(ctx, bs.CharacterID, buffID, buffsvc.BuffEdit{Duration: intPtr(2)})
			return err
		}},
		{name: "buff trigger", setup: activate, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.Trigger(ctx, bs.CharacterID, buffID)
			return err
		}},
		{name: "buff dismiss", setup: activate, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.Dismiss(ctx, bs.CharacterID, buffID)
			return err
		}},
		{name: "saved buff save", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.SaveBuff(ctx, bs.CharacterID, hammer())
			return err
		}},
		{name: "saved buff delete", setup: func(ctx context.Context, bs *composition.BackingServices) error {
			list, err := bs.Buffs.SaveBuff(ctx, bs.CharacterID, hammer())
			if err == nil {
				savedID = list[0].ID
			}
			return err
		}, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.DeleteSaved(ctx, bs.CharacterID, savedID)
			return err
		}},
		{name: "Next Round", setup: activate, run: advance},
		{name: "initiative reroll", setup: advance, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Buffs.RerollInitiative(ctx, bs.CharacterID)
			return err
		}},
		{name: "set round", setup: advance, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, _, err := bs.Buffs.SetRound(ctx, bs.CharacterID, 5)
			return err
		}},
		{name: "end combat", setup: advance, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, _, err := bs.Buffs.ResetRound(ctx, bs.CharacterID)
			return err
		}},
		{name: "pass time", setup: func(ctx context.Context, bs *composition.BackingServices) error {
			if err := activate(ctx, bs); err != nil {
				return err
			}
			return advance(ctx, bs)
		}, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, _, _, _, err := bs.Buffs.PassTime(ctx, bs.CharacterID, 1)
			return err
		}},
		{name: "attack", run: shot(new(5))},
		{name: "saving throw", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Combat.ResolveSavingThrow(ctx, bs.CharacterID, combatsvc.SaveSpell)
			return err
		}},
		{name: "skill check", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Skills.Roll(ctx, bs.CharacterID, "climb_walls")
			return err
		}},
		{name: "dice roll", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Dice.Roll(ctx, bs.CharacterID, "2d6+1")
			return err
		}},
		{name: "correction: save adjustment", setup: func(ctx context.Context, bs *composition.BackingServices) error {
			res, err := bs.Combat.ResolveSavingThrow(ctx, bs.CharacterID, combatsvc.SaveSpell)
			if err == nil {
				lastSave = res.LogEntryID
			}
			return err
		}, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Combat.AdjustSave(ctx, bs.CharacterID, lastSave, combatsvc.SaveAdjustment{Value: -3})
			return err
		}},
		{name: "correction: target AC", setup: shot(new(5)), run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Combat.AdjustAttackAC(ctx, bs.CharacterID, lastAttack, 2)
			return err
		}},
		{name: "correction: unknown AC", setup: shot(new(5)), run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Combat.ClearAttackAC(ctx, bs.CharacterID, lastAttack)
			return err
		}},
		{name: "correction: GM verdict", setup: shot(nil), run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Combat.SetAttackVerdict(ctx, bs.CharacterID, lastAttack, false)
			return err
		}},
		{name: "item use", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Items.UseItem(ctx, bs.CharacterID, "wand_lightning")
			return err
		}},
		{name: "item restock", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Items.Restock(ctx, bs.CharacterID, "arrows_plus3", 2)
			return err
		}},
		{name: "item create", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "Rope", Behaviour: itemsvc.BehaviourStock, Count: 1})
			return err
		}},
		{name: "item edit", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Items.Update(ctx, bs.CharacterID, "wand_lightning", itemsvc.ItemPatch{Count: ptr(3)})
			return err
		}},
		{name: "item delete", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Items.Delete(ctx, bs.CharacterID, "wand_lightning")
			return err
		}},
		{name: "weapon create", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Weapons.Create(ctx, bs.CharacterID, weaponsvc.WeaponInput{Name: "Long Bow", DamageSM: "1d8", DamageL: "1d8", Kind: "missile", TwoHanded: true, AmmoKind: "arrow"})
			return err
		}},
		{name: "weapon edit", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Weapons.Update(ctx, bs.CharacterID, "bastard_sword", weaponsvc.WeaponPatch{MagicToHit: ptr(3)})
			return err
		}},
		{name: "weapon delete", run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.Weapons.Delete(ctx, bs.CharacterID, "bastard_sword")
			return err
		}},
		{name: "xp add", setup: award, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 100, Reason: "riddle", Session: "S42"})
			return err
		}},
		{name: "xp edit", setup: award, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.XP.Update(ctx, bs.CharacterID, xpID, xpsvc.EntryPatch{Base: ptr(1100), Bonus: ptr(false), Reason: ptr("ogre")})
			return err
		}},
		{name: "xp delete", setup: award, run: func(ctx context.Context, bs *composition.BackingServices) error {
			_, err := bs.XP.Delete(ctx, bs.CharacterID, xpID)
			return err
		}},
	}
}

// ADR 0018 steps 1 and 2: one undo after any table-state mutation gives the
// whole state back (every stack table, HP, the log as it reads) and leaves
// the previous transaction on top. A service that records two entries in one
// request without sharing the transaction fails here: one undo would leave the
// first entry behind.
func TestUndo_OneUndoWalksEveryMutationFullyBack(t *testing.T) {
	for _, m := range stackMutations() {
		t.Run(m.name, func(t *testing.T) {
			bs := bootMaxDice(t)
			ctx := context.Background()
			// Something older on the stack, so "the previous top is back" is
			// not trivially true.
			_, err := bs.HP.AppendEvent(ctx, bs.CharacterID, -1, "scratch")
			require.NoError(t, err)
			if m.setup != nil {
				require.NoError(t, m.setup(ctx, bs))
			}
			before := tableStateOf(t, bs)
			beforeTop := topID(t, bs)

			require.NoError(t, m.run(ctx, bs))
			top := topID(t, bs)
			require.NotEqual(t, beforeTop, top, "the mutation recorded a transaction")

			popped, err := bs.Log.Undo(ctx, bs.CharacterID, top)
			require.NoError(t, err)
			assert.Equal(t, actionlog.StatusVoided, popped.Status())
			assert.Equal(t, before, tableStateOf(t, bs), "one undo walks the state fully back")
			assert.Equal(t, beforeTop, topID(t, bs), "the previous transaction is on top again")
		})
	}
}

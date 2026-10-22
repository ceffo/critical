package composition_test

// ADR 0043 steps 12, 14 and 15 end to end: maintenance counted on the row
// whatever the round counter does, undo and redo of what it charged and
// ended, and the second-power warning.

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

// fives are dice that roll 5 on every die: every check of the fixture's
// powers succeeds and every initiative is 5.
func fives(t *testing.T) *attackDice {
	return &attackDice{t: t, dies: slices.Repeat([]int{5}, 200), calls: map[string]int{}}
}

// auraSightKeeping is the fixture catalogue with Aura Sight's maintenance
// set to cost every period rounds.
func auraSightKeeping(t *testing.T, cost, period int) string {
	t.Helper()
	return writeArtifact(t, func(d map[string]any) {
		for _, p := range d["powers"].([]any) {
			if p := p.(map[string]any); p["id"] == "aura_sight" {
				p["maintenance"] = map[string]any{"cost": cost, "period": period}
			}
		}
	})
}

var auraSight = []psionics.Pick{
	{Kind: psionics.PickDiscipline, ID: "clairsentience"}, {Kind: psionics.PickPower, ID: "aura_sight"},
}

// maintainedRows are the active rows kept up by a power.
func maintainedRows(t *testing.T, buffs buffsvc.SVC, id int64) []buffsvc.BuffEffect {
	t.Helper()
	rows, err := buffs.ListActive(context.Background(), id)
	require.NoError(t, err)
	return slices.DeleteFunc(rows, func(b buffsvc.BuffEffect) bool { return !b.Maintained() })
}

// Reviewer's scenarios 1 and 2: Aura Sight (9 PSPs, here 1 a round) used in
// round 5, then End combat. End combat closes its activation round, so each
// of the next three Next Rounds charges, and an hour passed afterwards
// charges all 60 rounds, as its preview said.
func TestMaintenance_Step14_CountedOnTheRowAcrossEndCombat(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistOn(t, auraSight, fives(t), auraSightKeeping(t, 1, 1))
	_, _, err := bs.Buffs.SetRound(ctx, id, 5)
	require.NoError(t, err)
	res, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	require.True(t, res.Check.Success)
	require.Equal(t, 172, currentPSP(t, bs, id))

	_, _, err = bs.Buffs.ResetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 172, currentPSP(t, bs, id), "End combat charges nothing")
	for range 3 {
		_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}
	assert.Equal(t, 169, currentPSP(t, bs, id), "3 Next Rounds after the round counter went back to 0: 3 charges")

	rounds, expiring, spent, err := bs.Buffs.PreviewPassTime(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, 60, rounds)
	assert.Empty(t, expiring)
	assert.Equal(t, 60, spent, "the preview's PSPs")
	_, _, _, _, _, err = bs.Buffs.PassTime(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, 109, currentPSP(t, bs, id), "an hour: 60 charges, round 0 included")
	assert.Len(t, maintainedRows(t, bs.Buffs, id), 1)
}

// Passing time straight after End combat in the activation round charges
// every one of its rounds too.
func TestMaintenance_Step15_PassTimeAfterEndCombat(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistOn(t, auraSight, fives(t), auraSightKeeping(t, 1, 1))
	_, _, err := bs.Buffs.SetRound(ctx, id, 8)
	require.NoError(t, err)
	_, err = bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	_, _, err = bs.Buffs.ResetRound(ctx, id)
	require.NoError(t, err)

	_, _, _, _, _, err = bs.Buffs.PassTime(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, 172-60, currentPSP(t, bs, id))

	_, _, _, _, _, err = bs.Buffs.PassTime(ctx, id, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, currentPSP(t, bs, id), "112 more charges, then unpaid")
	assert.Empty(t, maintainedRows(t, bs.Buffs, id), "it ends at the first charge it cannot pay")
}

// Reviewer's scenario 3: a Next Round the PSPs cannot pay ends the power;
// undo puts the row and the PSPs back, redo ends it again.
func TestMaintenance_Step14_UnpaidNextRoundUndoRedo(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, auraSight, fives(t))
	_, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id) // the activation round: free
	require.NoError(t, err)
	spendPSP(t, bs, id, 167)
	require.Equal(t, 5, currentPSP(t, bs, id))

	_, expired, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "aura_sight", expired[0].PowerID)
	assert.Empty(t, maintainedRows(t, bs.Buffs, id))
	assert.Equal(t, 5, currentPSP(t, bs, id), "nothing spent on the unpaid power")
	entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindRoundAdvance, 1)
	require.NoError(t, err)
	assert.Contains(t, string(entries[0].Payload), `"maintenanceUnpaid":["aura_sight"]`)

	undoTop(t, bs, id)
	rows := maintainedRows(t, bs.Buffs, id)
	require.Len(t, rows, 1, "the undo puts the row back")
	assert.Equal(t, 1, rows[0].RoundsKept, "with its count")
	assert.Equal(t, 5, currentPSP(t, bs, id))

	redoTop(t, bs, id)
	assert.Empty(t, maintainedRows(t, bs.Buffs, id), "the redo ends it again")
	assert.Equal(t, 5, currentPSP(t, bs, id))
}

// Reviewer's scenario 4: undoing a maintained power's check takes the row
// and the spend back; redoing it puts both back, rolling no new d20. A
// charging Next Round redone charges again.
func TestMaintenance_Step13_CheckAndChargeUndoRedo(t *testing.T) {
	ctx := context.Background()
	dice := fives(t)
	bs, id := thiefPsionicistRolling(t, auraSight, dice)
	_, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, dice.calls["die"])

	undoTop(t, bs, id)
	assert.Empty(t, maintainedRows(t, bs.Buffs, id))
	assert.Equal(t, 181, currentPSP(t, bs, id))
	redoTop(t, bs, id)
	assert.Len(t, maintainedRows(t, bs.Buffs, id), 1, "the row is back")
	assert.Equal(t, 172, currentPSP(t, bs, id), "and the spend")
	assert.Equal(t, 1, dice.calls["die"], "no new d20")

	for range 2 {
		_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}
	require.Equal(t, 163, currentPSP(t, bs, id))
	undoTop(t, bs, id)
	assert.Equal(t, 172, currentPSP(t, bs, id))
	redoTop(t, bs, id)
	assert.Equal(t, 163, currentPSP(t, bs, id), "the redone Next Round charges again")
	assert.Equal(t, 2, maintainedRows(t, bs.Buffs, id)[0].RoundsKept)
}

// Reviewer's scenario 5: using a maintained power again renews its row (one
// row) and starts its count again: a period-10 power's next charge is the
// Next Round ending its 10th round kept after the new activation round.
func TestMaintenance_Step13_ReactivationResetsTheCount(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistOn(t, auraSight, fives(t), auraSightKeeping(t, 9, 10))
	_, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	for range 6 {
		_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}
	require.Equal(t, 6, maintainedRows(t, bs.Buffs, id)[0].RoundsKept)

	_, err = bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	rows := maintainedRows(t, bs.Buffs, id)
	require.Len(t, rows, 1, "one row")
	assert.Equal(t, 0, rows[0].RoundsKept, "its count starts again")
	require.Equal(t, 163, currentPSP(t, bs, id))

	for n := 1; n <= 10; n++ {
		_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
		require.Equal(t, 163, currentPSP(t, bs, id), "Next Round %d after it", n)
	}
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 154, currentPSP(t, bs, id), "10 rounds kept after the activation round: charged")
}

// Reviewer's scenario 7: in one combat round, a defence mode then two other
// powers: only the third check warns; nothing warns at round 0.
func TestUse_Step12_SecondPowerOfTheRound(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"},
		{Kind: psionics.PickPower, ID: "psionic_blast"},
		{Kind: psionics.PickPower, ID: "mind_blank"},
	}, fives(t))
	use := func(power string) []string {
		t.Helper()
		res, err := bs.Psionics.Use(ctx, id, power, psionicsvc.UseRequest{})
		require.NoError(t, err)
		return res.Check.Warnings
	}

	assert.Empty(t, use("psionic_blast"))
	assert.Empty(t, use("psionic_blast"), "round 0: out of combat")

	_, _, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, use("mind_blank"))
	assert.Empty(t, use("psionic_blast"), "a defence mode does not count")
	assert.Equal(t, []string{psionicsvc.WarnSecondPowerThisRound}, use("psionic_blast"))

	_, _, err = bs.Buffs.ResetRound(ctx, id)
	require.NoError(t, err)
	_, _, err = bs.Buffs.SetRound(ctx, id, 1)
	require.NoError(t, err)
	assert.Empty(t, use("psionic_blast"), "a Set round starts a new round")
}

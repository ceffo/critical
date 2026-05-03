package domain_test

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvanceRoundAtomic(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	// Apply one duration buff and one charge buff
	require.NoError(t, actor.ApplyBuff(domain.BuffEffect{Name: "Bless", Type: "duration", Duration: 1}))
	require.NoError(t, actor.ApplyBuff(domain.BuffEffect{Name: "Shield", Type: "duration", Duration: 3}))
	require.NoError(t, actor.ApplyBuff(domain.BuffEffect{Name: "Power Strike", Type: "charge", Charges: 2}))

	active, expired, round, err := domain.AdvanceRound(db, charID)
	require.NoError(t, err)

	assert.Equal(t, 1, round)
	assert.Len(t, active, 2, "Shield + Power Strike should survive")
	assert.Len(t, expired, 1, "Bless (Duration 1) should expire")
	assert.Equal(t, "Bless", expired[0].Name)

	// Round counter persisted
	savedRound, err := domain.GetRoundNumber(db, charID)
	require.NoError(t, err)
	assert.Equal(t, 1, savedRound)

	// Active buffs persisted — Bless gone, Shield at Duration 2
	reloaded, err := domain.LoadActor(db, charID)
	require.NoError(t, err)
	assert.Len(t, reloaded.ActiveBuffs, 2)

	var shieldBuff domain.BuffEffect
	for _, b := range reloaded.ActiveBuffs {
		if b.Name == "Shield" {
			shieldBuff = b
		}
	}
	assert.Equal(t, 2, shieldBuff.Duration)
}

func TestApplyBuffAssignsID(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	// Supply a buff with no ID; domain should assign one
	buff := domain.BuffEffect{Name: "Bless", Type: "duration", Duration: 3}
	require.NoError(t, actor.ApplyBuff(buff))
	require.Len(t, actor.ActiveBuffs, 1)
	assert.NotEmpty(t, actor.ActiveBuffs[0].ID, "ApplyBuff must assign a UUID when ID is empty")
}

func TestApplyBuffPreservesExplicitID(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	buff := domain.BuffEffect{ID: "explicit-id", Name: "Haste", Type: "duration", Duration: 5}
	require.NoError(t, actor.ApplyBuff(buff))
	assert.Equal(t, "explicit-id", actor.ActiveBuffs[0].ID)
}

func TestDismissBuff(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	buff := domain.BuffEffect{ID: "dismiss-me", Name: "Bless", Type: "duration", Duration: 3}
	require.NoError(t, actor.ApplyBuff(buff))
	require.Len(t, actor.ActiveBuffs, 1)

	require.NoError(t, actor.DismissBuff("dismiss-me"))
	assert.Empty(t, actor.ActiveBuffs)

	// Verify persistence: reload from DB
	reloaded, err := domain.LoadActor(db, charID)
	require.NoError(t, err)
	assert.Empty(t, reloaded.ActiveBuffs)
}

func TestTriggerBuff(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	buff := domain.BuffEffect{ID: "shield", Name: "Shield of Faith", Type: "charge", Charges: 2}
	require.NoError(t, actor.ApplyBuff(buff))

	// First trigger: charges → 1, buff still active
	require.NoError(t, actor.TriggerBuff("shield"))
	require.Len(t, actor.ActiveBuffs, 1)
	assert.Equal(t, 1, actor.ActiveBuffs[0].Charges)

	// Second trigger: charges → 0, buff removed
	require.NoError(t, actor.TriggerBuff("shield"))
	assert.Empty(t, actor.ActiveBuffs)

	// Verify persistence
	reloaded, err := domain.LoadActor(db, charID)
	require.NoError(t, err)
	assert.Empty(t, reloaded.ActiveBuffs)
}

func TestDismissNonexistentBuff(t *testing.T) {
	db, charID := openTestDB(t)

	actor, err := domain.LoadActor(db, charID)
	require.NoError(t, err)

	// Dismissing a non-existent ID is a no-op, not an error
	require.NoError(t, actor.DismissBuff("ghost-id"))
	assert.Empty(t, actor.ActiveBuffs)
}

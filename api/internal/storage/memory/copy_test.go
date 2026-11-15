package memory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ADR 0029 step 12: the copy of the seeded character holds a copy of every table's rows but the
// action log, under a new id, owned by the new owner. Each assertion names the table it checks.
func TestCopyCharacter_Step12_EveryTableButTheLogIsCopied(t *testing.T) {
	s, kelsen := seeded(t)
	ctx := context.Background()
	writeEverything(t, ctx, s, kelsen)
	initiative := &buffsvc.Initiative{D10: 4, LogEntryID: 99}
	require.NoError(t, s.Buffs().SaveInitiative(ctx, kelsen, initiative))
	require.NoError(t, s.Buffs().ReplaceSavedBuffs(ctx, kelsen, []buffsvc.SavedBuff{{Name: "Bless", Duration: 6, Charges: 1, Note: "n"}}))
	ac := 3
	require.NoError(t, s.Forms().Put(ctx, kelsen, combatsvc.AttackForm{WeaponID: "longsword", TargetAC: &ac, TargetSize: "l", Situational: combatsvc.Situational{Value: 2, Label: "flank"}}))
	require.NoError(t, s.Spellbooks().SetLearnFailure(ctx, kelsen, "sleep", 3))
	require.NoError(t, s.Characters().SetPSPMax(ctx, kelsen, 40))
	before := read(t, s, kelsen)
	require.NotEmpty(t, before.Log, "the source has a history")
	require.NotEmpty(t, before.Items)
	require.NotEmpty(t, before.XP)

	newOwner := authsvc.User{ID: uuid.New(), Email: "new@example.com", Role: authsvc.RolePlayer}
	require.NoError(t, s.Auth().InsertUser(ctx, newOwner))
	id, err := s.Characters().CopyCharacter(ctx, newOwner.ID, kelsen, "Kelsen (copy)", charactersvc.Openings{HP: -6, PSP: -11})
	require.NoError(t, err)
	require.NotEqual(t, kelsen, id)

	got := read(t, s, id)
	assert.Empty(t, got.Log, "action_log: no history")
	assert.Equal(t, before.Buffs, got.Buffs, "active_buffs")
	assert.Equal(t, before.Round, got.Round, "round_state")
	assert.Equal(t, before.Slots, got.Slots, "spell_slots")
	assert.Equal(t, before.Items, got.Items, "items, ids kept")
	assert.Equal(t, before.Reviews, got.Reviews, "equipment_review")
	assert.Equal(t, before.ACOverride, got.ACOverride, "ac override")
	require.Len(t, got.XP, len(before.XP), "xp_entries")
	for i := range before.XP {
		want := before.XP[i]
		want.ID = got.XP[i].ID
		assert.Equal(t, want, got.XP[i], "xp entry %d, under its own id", i)
		assert.NotEqual(t, before.XP[i].ID, got.XP[i].ID)
	}
	assert.Equal(t, before.Favourite, got.Favourite, "dice_favourites")

	init2, err := s.Buffs().GetInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, initiative, init2, "round_state initiative")
	saved, err := s.Buffs().ListSavedBuffs(ctx, id)
	require.NoError(t, err)
	require.Len(t, saved, 1, "saved_buffs")
	assert.Equal(t, "Bless", saved[0].Name)
	assert.Equal(t, 6, saved[0].Duration)
	srcSaved, err := s.Buffs().ListSavedBuffs(ctx, kelsen)
	require.NoError(t, err)
	assert.NotEqual(t, srcSaved[0].ID, saved[0].ID)
	form, ok, err := s.Forms().Get(ctx, id)
	require.NoError(t, err)
	require.True(t, ok, "attack_forms")
	assert.Equal(t, "longsword", form.WeaponID)
	assert.Equal(t, "flank", form.Situational.Label)
	srcBook, err := s.Spellbooks().Entries(ctx, kelsen)
	require.NoError(t, err)
	book, err := s.Spellbooks().Entries(ctx, id)
	require.NoError(t, err)
	assert.NotEmpty(t, book)
	assert.Equal(t, srcBook, book, "spellbook_entries, origins kept")
	level, found, err := s.Spellbooks().LearnFailure(ctx, id, "sleep")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 3, level, "spellbook_learn_failures")

	src, err := s.Characters().GetCharacter(ctx, kelsen)
	require.NoError(t, err)
	cp, err := s.Characters().GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Kelsen (copy)", cp.Name)
	assert.Equal(t, id, cp.ID)
	cp.ID, cp.Name = src.ID, src.Name
	assert.Equal(t, src, cp, "characters: the sheet, the picks, the class entries, PSP maximum")

	// The ledgers: the copy starts at its opening balance; the source's totals are the log's.
	hp, err := s.ActionLog().HPTotal(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -6, hp)
	psp, err := s.ActionLog().PSPTotal(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -11, psp)
	srcHP, err := s.ActionLog().HPTotal(ctx, kelsen)
	require.NoError(t, err)
	assert.Zero(t, srcHP, "the source has no opening balance: its total is its log's")

	// Whose: the new owner's, not current, listed with the owner's characters.
	mine, err := s.Characters().CharactersOf(ctx, newOwner.ID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, id, mine[0].ID)
	cur, err := s.Characters().CurrentCharacter(ctx, newOwner.ID)
	require.NoError(t, err)
	assert.Zero(t, cur, "a copy is not made current")

	// The source is as it was, and the two are independent.
	assert.Equal(t, before, read(t, s, kelsen), "the source is unchanged")
	require.NoError(t, s.Buffs().SaveRoundNumber(ctx, id, 20))
	require.NoError(t, s.Items().Delete(ctx, id, before.Items[0].ID))
	require.NoError(t, s.Characters().SetACOverride(ctx, id, &acsvc.Override{Value: -2, Label: "other"}))
	require.NoError(t, s.Spellbooks().AddEntry(ctx, id, spellbook.Entry{Key: "web", Origin: spellbook.OriginLearned}))
	assert.Equal(t, before, read(t, s, kelsen), "a write on the copy leaves the source")
	require.NoError(t, s.Buffs().SaveRoundNumber(ctx, kelsen, 3))
	assert.Equal(t, 20, read(t, s, id).Round, "a write on the source leaves the copy")
}

// Step 12.1: the source is read as ViewableCharacter reads it; an unreadable one is not found and
// nothing is created; a rollback takes the copy back.
func TestCopyCharacter_Step12_UnreadableSourceIsNotFound(t *testing.T) {
	s, kelsen := seeded(t)
	ctx := context.Background()
	owner := authsvc.User{ID: uuid.New(), Email: "o@example.com", Role: authsvc.RolePlayer}
	require.NoError(t, s.Auth().InsertUser(ctx, owner))
	other, err := s.AddCharacterOf(ctx, owner.ID, charactersvc.Character{Name: "Other", Class: "bard"})
	require.NoError(t, err)
	require.NoError(t, s.RetireCharacter(ctx, other))
	ownerless, err := s.AddCharacter(ctx, charactersvc.Character{Name: "Nobody's", Class: "bard"})
	require.NoError(t, err)

	for name, source := range map[string]int64{"unknown": 9999, "retired": other, "no owner": ownerless} {
		_, err := s.Characters().CopyCharacter(ctx, owner.ID, source, "x", charactersvc.Openings{})
		assert.ErrorIs(t, err, domain.ErrCharacterNotFound, name)
	}
	disabled := authsvc.User{ID: uuid.New(), Email: "d@example.com", Role: authsvc.RolePlayer, Disabled: true}
	require.NoError(t, s.Auth().InsertUser(ctx, disabled))
	theirs, err := s.AddCharacterOf(ctx, disabled.ID, charactersvc.Character{Name: "Theirs", Class: "bard"})
	require.NoError(t, err)
	_, err = s.Characters().CopyCharacter(ctx, owner.ID, theirs, "x", charactersvc.Openings{})
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound, "disabled owner")

	mine, err := s.Characters().CharactersOf(ctx, owner.ID)
	require.NoError(t, err)
	assert.Empty(t, mine, "nothing was created")

	require.ErrorIs(t, s.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.Characters().CopyCharacter(ctx, owner.ID, kelsen, "x", charactersvc.Openings{}); err != nil {
			return err
		}
		return errBoom
	}), errBoom)
	mine, err = s.Characters().CharactersOf(ctx, owner.ID)
	require.NoError(t, err)
	assert.Empty(t, mine, "a rolled-back copy leaves nothing")
}

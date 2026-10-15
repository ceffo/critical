package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The details on the real stack: they start empty, a partial patch changes
// only its fields, a rejected patch writes nothing, and the values survive a
// restart and come back with the character.
func TestCharacterDetails_EditPersistsOnTheRealStack(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "critical.db")
	seeds := filepath.Join("..", "..", "testdata", "seeds")
	open := func() *composition.BackingServices {
		bs, err := composition.New(ctx, composition.Config{DSN: dsn, SeedsDir: seeds})
		require.NoError(t, err)
		return bs
	}
	s := func(v string) *string { return &v }

	bs := open()
	c, err := bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, "bard", c.Class)
	assert.Empty(t, c.Details.Race)
	assert.Empty(t, c.Details.Languages)

	age := 120
	_, err = bs.Details.Update(ctx, bs.CharacterID, detailssvc.Patch{Race: s("Half-elf"), Age: &age, Languages: &[]string{"Commun", "Elfique"}})
	require.NoError(t, err)
	_, err = bs.Details.Update(ctx, bs.CharacterID, detailssvc.Patch{Eyes: s("green")})
	require.NoError(t, err)

	_, err = bs.Details.Update(ctx, bs.CharacterID, detailssvc.Patch{ClearAge: true})
	require.NoError(t, err)
	c, err = bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Zero(t, c.Details.Age, "a cleared age is not set")
	_, err = bs.Details.Update(ctx, bs.CharacterID, detailssvc.Patch{Age: &age})
	require.NoError(t, err)

	bad := 10_001
	_, err = bs.Details.Update(ctx, bs.CharacterID, detailssvc.Patch{Hair: s("black"), Age: &bad})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	require.NoError(t, bs.Close(ctx))
	bs = open()
	defer func() { _ = bs.Close(ctx) }()
	c, err = bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, "Half-elf", c.Details.Race)
	assert.Equal(t, 120, c.Details.Age)
	assert.Equal(t, "green", c.Details.Eyes)
	assert.Empty(t, c.Details.Hair, "the rejected patch wrote nothing")
	assert.Equal(t, []string{"Commun", "Elfique"}, c.Details.Languages)
}

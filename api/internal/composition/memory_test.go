package composition_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
)

// ADR 0025 step 8.1: with InMemory, New wires the in-memory storage, seeded
// from the seeds and the catalogue, and opens no database: no file appears
// at the DSN, and the services serve the fixture's character.
func TestNew_Step8_1_InMemoryOpensNoDatabase(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "critical.db")
	cfg := fixtureConfig()
	cfg.DSN, cfg.InMemory = dsn, true

	bs, err := composition.New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	assert.Nil(t, bs.DB, "no SQLite handle in memory")
	_, err = os.Stat(dsn)
	assert.ErrorIs(t, err, os.ErrNotExist, "nothing is created at the DSN")

	ch, err := bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, "Test Bard", ch.Name)
	hp, err := bs.HP.GetState(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, 85, hp.CurrentHp)
}

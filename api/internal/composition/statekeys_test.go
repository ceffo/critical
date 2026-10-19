package composition_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0018 step 11 through the real services: each write names the state
// keys of the transaction it committed, and the undo and the redo of it name
// the same keys (ADR 0019: two more actions).
func TestStateKeys_EachWriteNamesWhatItChanged(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	keysOf := func(write func(ctx context.Context) error) []string {
		t.Helper()
		ctx, changes := actionlog.WithChanges(ctx)
		require.NoError(t, write(ctx))
		return changes.Keys()
	}

	assert.Equal(t, []string{"hp", "log"}, keysOf(func(ctx context.Context) error {
		_, err := bs.Log.AppendHPEvent(ctx, id, -3, "an orc")
		return err
	}), "an HP change")

	used := keysOf(func(ctx context.Context) error {
		_, err := bs.Items.UseItem(ctx, id, "fire_flask")
		return err
	})
	assert.Equal(t, []string{"items:fire_flask", "log"}, used, "a use spends the flask")

	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, used, keysOf(func(ctx context.Context) error {
		_, err := bs.Log.Undo(ctx, id, top)
		return err
	}), "the undo names what the use changed")
	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, redo)
	assert.Equal(t, used, keysOf(func(ctx context.Context) error {
		_, err := bs.Log.Redo(ctx, id, redo.ID)
		return err
	}), "and so does the redo")

	assert.Equal(t, []string{"buffs", "initiative", "log", "round"}, keysOf(func(ctx context.Context) error {
		_, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
		return err
	}), "Next Round: the round, the buffs' expiry and the initiative")

	assert.Empty(t, keysOf(func(ctx context.Context) error {
		_, err := bs.Log.Undo(ctx, id, -1)
		if err == nil {
			t.Fatal("a stale undo is refused")
		}
		return nil
	}), "a refused write names nothing")
}

// ADR 0018 step 11: the web's invalidation map (web/src/lib/stateKeys.ts,
// exhaustive by type over STATE_FAMILIES) knows every family a write can
// name: the log's own two and every family a module snapshots. A family the
// web does not know reads every slice again, so a missing one costs requests,
// never a stale panel; this keeps the lists equal.
func TestStateKeys_TheWebMapsEveryFamily(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)

	server := []string{actionlog.FamilyLog, actionlog.FamilyHP}
	for _, svc := range composition.Services(bs) {
		for _, c := range svc.Module().Codecs {
			server = append(server, c.Family())
		}
	}
	slices.Sort(server)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "lib", "stateKeys.ts"))
	require.NoError(t, err)
	m := regexp.MustCompile(`(?s)export const STATE_FAMILIES = \[(.*?)\] as const`).FindSubmatch(raw)
	require.NotNil(t, m, "STATE_FAMILIES not found in web/src/lib/stateKeys.ts")
	var web []string
	for _, q := range regexp.MustCompile(`"([a-z_]+)"`).FindAllSubmatch(m[1], -1) {
		web = append(web, string(q[1]))
	}
	slices.Sort(web)
	assert.Equal(t, server, web)
}

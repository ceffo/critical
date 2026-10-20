package composition_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	compmocks "github.com/ceffo/critical/api/internal/composition/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	actionlogmocks "github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
	authmocks "github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
	combatmocks "github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	dicemocks "github.com/ceffo/critical/api/internal/domain/service/dicesvc/mocks"
	itemmocks "github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	xpmocks "github.com/ceffo/critical/api/internal/domain/service/xpsvc/mocks"
)

var (
	fixtureCatalogue = filepath.Join("..", "..", "testdata", "catalogue.json")
	fixtureSeeds     = filepath.Join("..", "..", "testdata", "seeds")
)

const characterID = int64(7)

// harness is the root's storage as mocks: every port of composition.Storage
// is a mockery mock whose unexpected call fails the test, so a test names
// every operation the code under test may call (ADR 0025 step 6.1).
type harness struct {
	Storage composition.Storage

	Store     *compmocks.MockStore
	Character *compmocks.MockCharacterStore
	HP        *actionlogmocks.MockMaxHP
	Log       *actionlogmocks.MockDAL
	Buff      *compmocks.MockBuffStore
	Spell     *compmocks.MockSpellStore
	Crit      *combatmocks.MockCritStore
	Items     *itemmocks.MockDAL
	Forms     *combatmocks.MockFormStore
	XP        *xpmocks.MockDAL
	Spellbook *compmocks.MockSpellbookStore
	Dice      *dicemocks.MockDAL
	Auth      *authmocks.MockDAL
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		Store:     compmocks.NewMockStore(t),
		Character: compmocks.NewMockCharacterStore(t),
		HP:        actionlogmocks.NewMockMaxHP(t),
		Log:       actionlogmocks.NewMockDAL(t),
		Buff:      compmocks.NewMockBuffStore(t),
		Spell:     compmocks.NewMockSpellStore(t),
		Crit:      combatmocks.NewMockCritStore(t),
		Items:     itemmocks.NewMockDAL(t),
		Forms:     combatmocks.NewMockFormStore(t),
		XP:        xpmocks.NewMockDAL(t),
		Spellbook: compmocks.NewMockSpellbookStore(t),
		Dice:      dicemocks.NewMockDAL(t),
		Auth:      authmocks.NewMockDAL(t),
	}
	h.Storage = composition.Storage{
		Store: h.Store, Character: h.Character, HP: h.HP, Log: h.Log, Buff: h.Buff, Spell: h.Spell,
		Crit: h.Crit, Items: h.Items, Forms: h.Forms,
		XP: h.XP, Spellbook: h.Spellbook, Dice: h.Dice, Auth: h.Auth,
	}
	// Every service runs its transactions through the store: the body runs
	// once, in the caller's context.
	h.Store.EXPECT().InTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
	return h
}

// expectStartup expects the steps a successful start takes on the store, in
// order, ending with the root admin's adoption of the characters without an
// owner (no root admin here: nothing to adopt).
func (h *harness) expectStartup(seedsDir string) {
	migrate := h.Store.EXPECT().Migrate(mock.Anything, seedsDir, mock.Anything).Return(nil).Once()
	seed := h.Store.EXPECT().Seed(mock.Anything, seedsDir, mock.Anything, mock.Anything).Return(nil).Once().NotBefore(migrate)
	imp := h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once().NotBefore(seed)
	search := h.Spell.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, nil).Once().NotBefore(imp)
	h.Spell.EXPECT().Details(mock.Anything, []string{}).Return(nil, nil).Once().NotBefore(search)
	h.Auth.EXPECT().RootUser(mock.Anything).Return(nil, domain.ErrNotFound).Once().NotBefore(imp)
}

func (h *harness) build(t *testing.T, cfg composition.Config) (*composition.BackingServices, error) {
	t.Helper()
	return composition.Build(context.Background(), h.Storage, cfg)
}

func fixtureConfig() composition.Config {
	return composition.Config{SeedsDir: fixtureSeeds, Catalogue: fixtureCatalogue}
}

// writeArtifact writes the fixture catalogue, changed by mutate, to a temp
// file and returns its path.
func writeArtifact(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	raw, err := os.ReadFile(fixtureCatalogue)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	mutate(doc)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "catalogue.json")
	require.NoError(t, os.WriteFile(path, out, 0o600))
	return path
}

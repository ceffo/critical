package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0030 step 20.7 through the wiring, the fixture catalogue and the in-memory store: the
// options offer a mage Detect Magic pre-ticked (the fixture holds no Read Magic) and the
// catalogue's wizard spells; a level 1 mage created with Detect Magic and Sleep has both in his
// book, in that order, with the origin starting. No die is rolled (the dice refuse every roll)
// and the journal holds nothing. A 2nd-level spell is refused at level 1, nothing stored.
func TestCreate_Step20_7_StartingSpellsWithoutADieThroughTheWiring(t *testing.T) {
	ctx := context.Background()
	dice := &d100s{t: t}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)

	opts, err := bs.Character.CreationOptions(ctx)
	require.NoError(t, err)
	for _, c := range opts.Classes {
		if c.Class == "mage" {
			assert.Equal(t, []string{"detect_magic"}, c.Spellbook.StartingSpells)
		}
	}
	require.NotEmpty(t, opts.WizardSpells)
	assert.Contains(t, opts.WizardSpells, charactersvc.SpellOption{ID: "sleep", Level: 1})

	mage := charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Elminster", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 11, Int: 15, Wis: 12, Cha: 10, HPMax: 3,
	}
	before, err := bs.Character.List(ctx, s.User.ID)
	require.NoError(t, err)
	refused := mage
	refused.Spellbook = []string{"sleep", "mirror_image"}
	_, err = bs.Character.Create(ctx, s.User.ID, refused)
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, domain.FieldError{Field: "spellbook.mirror_image", Message: "spell_level_above_class"}, *fe)
	list, err := bs.Character.List(ctx, s.User.ID)
	require.NoError(t, err)
	assert.Equal(t, before, list, "nothing stored")

	mage.Spellbook = []string{"detect_magic", "sleep"}
	id, err := bs.Character.Create(ctx, s.User.ID, mage)
	require.NoError(t, err)
	assert.Equal(t, []string{"detect_magic:starting", "sleep:starting"}, originsOf(t, bs, id))
	assert.Zero(t, dice.calls, "no die")
	entries, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)
	assert.Empty(t, entries, "no journal entry beyond the creation")
}

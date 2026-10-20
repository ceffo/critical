package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// reads counts the storage reads a request costs: the character row, the XP ledger and the
// item lists and gets (ADR 0025 step 10: a read costs a fixed, small number of queries).
type reads struct{ character, xp, items int }

func (r reads) total() int { return r.character + r.xp + r.items }

type countedCharacters struct {
	composition.CharacterStore
	n *reads
}

func (c countedCharacters) GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error) {
	c.n.character++
	return c.CharacterStore.GetCharacter(ctx, id)
}

type countedXP struct {
	xpsvc.DAL
	n *reads
}

func (x countedXP) List(ctx context.Context, characterID int64) ([]xpsvc.Entry, error) {
	x.n.xp++
	return x.DAL.List(ctx, characterID)
}

type countedItems struct {
	itemsvc.DAL
	n *reads
}

func (i countedItems) List(ctx context.Context, characterID int64) ([]itemsvc.Item, error) {
	i.n.items++
	return i.DAL.List(ctx, characterID)
}

func (i countedItems) Get(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error) {
	i.n.items++
	return i.DAL.Get(ctx, characterID, itemID)
}

// ADR 0037 step 9 costs one character row per weapon read, not a derived character (no XP
// ledger, no worn armour; ADR 0025 step 10): what the sheet's requests read, for a fighter and for a bard (whose derived character would also read
// the worn armour), each holding a long sword. The requests are the handlers' service calls: GET
// items (List, Warnings), GET weapons, GET the attack form, and an equip click (SetState,
// Warnings). The bard's item list and attack form still serve the sword.
func TestReads_Step9_SpecializationReadsOneCharacterRow(t *testing.T) {
	for _, tc := range []struct {
		class string
		want  map[string]reads
	}{
		// Each request reads what it did before step 9 plus one character row per read holding a
		// weapon (before step 9: items 1/1/1, weapons 0/0/1, equip 1/1/2, form 1/1/2 for the
		// fighter; the bard's derived character also lists his pieces for the worn armour).
		{"fighter", map[string]reads{
			"items":   {character: 2, xp: 1, items: 1},
			"weapons": {character: 1, items: 1},
			"equip":   {character: 3, xp: 1, items: 2},
			"form":    {character: 3, xp: 1, items: 2},
		}},
		{"bard", map[string]reads{
			"items":   {character: 2, xp: 1, items: 2},
			"weapons": {character: 1, items: 1},
			"equip":   {character: 3, xp: 1, items: 3},
			"form":    {character: 3, xp: 1, items: 3},
		}},
	} {
		t.Run(tc.class, func(t *testing.T) {
			ctx := context.Background()
			n := &reads{}
			st := composition.MemoryStorage(memory.New())
			st.Character = countedCharacters{st.Character, n}
			st.XP = countedXP{st.XP, n}
			st.Items = countedItems{st.Items, n}
			bs, err := composition.Build(ctx, st, fixtureConfig())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
			s, err := bs.Auth.DevLogin(ctx, "")
			require.NoError(t, err)
			id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
				Details: identity(),
				Name:    "Bram", Class: tc.class, Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
			})
			require.NoError(t, err)
			sword, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword"})
			require.NoError(t, err)

			got := map[string]reads{}
			count := func(name string, request func()) {
				*n = reads{}
				request()
				got[name] = *n
			}
			count("items", func() {
				items, err := bs.Items.List(ctx, id)
				require.NoError(t, err)
				require.Len(t, items, 1)
				_, err = bs.Items.Warnings(ctx, id, items)
				require.NoError(t, err)
			})
			count("weapons", func() {
				weapons, err := bs.Combat.ListWeapons(ctx, id)
				require.NoError(t, err)
				require.Len(t, weapons, 1)
			})
			count("equip", func() {
				_, err := bs.Items.SetState(ctx, id, sword.Item.ID, itemsvc.StateWielded, false)
				require.NoError(t, err)
				_, err = bs.Items.Warnings(ctx, id, nil)
				require.NoError(t, err)
			})
			count("form", func() {
				form, err := bs.Combat.GetAttackForm(ctx, id)
				require.NoError(t, err)
				require.NotNil(t, form.Preview, "the wielded sword's attack")
			})
			for name, r := range got {
				t.Logf("%s %s: %d reads (character %d, xp %d, items %d)", tc.class, name, r.total(), r.character, r.xp, r.items)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

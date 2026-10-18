package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// sheet is what a character owns, read through the services: what another
// character's play must never change.
type sheet struct {
	Items     []itemsvc.Item
	Weapons   []combatsvc.Weapon
	Skills    []string
	Spellbook []prepsvc.SpellbookEntry
	Mine      []string
}

func sheetOf(t *testing.T, bs *composition.BackingServices, characterID int64) sheet {
	t.Helper()
	ctx := context.Background()
	var s sheet
	var err error
	s.Items, err = bs.Items.List(ctx, characterID)
	require.NoError(t, err)
	s.Weapons, err = bs.Combat.ListWeapons(ctx, characterID)
	require.NoError(t, err)
	skills, err := bs.Skills.List(ctx, characterID)
	require.NoError(t, err)
	s.Skills = []string{}
	for _, p := range skills.Proficiencies {
		s.Skills = append(s.Skills, p.ID)
	}
	for _, b := range skills.BardSkills {
		s.Skills = append(s.Skills, b.ID)
	}
	info, err := bs.Prep.Info(ctx, characterID)
	require.NoError(t, err)
	s.Spellbook = info.Spellbook
	page, err := bs.Spells.Search(ctx, characterID, spellsvc.Filter{Class: "wizard", Mine: true},
		spellsvc.PageRequest{PageSize: spellsvc.MaxPageSize})
	require.NoError(t, err)
	s.Mine = []string{}
	for _, sp := range page.Items {
		s.Mine = append(s.Mine, sp.ID)
	}
	return s
}

// twoCharacters is the root over the in-memory storage, seeded from the
// fixture (its character owns the starting set), plus a second character that
// owns nothing.
func twoCharacters(t *testing.T) (bs *composition.BackingServices, seeded, other int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	other, err = m.AddCharacter(ctx, charactersvc.Character{
		Name: "Other", Class: "bard", Level: 1, Str: 10, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10, HPMax: 6, Thac0Base: 20,
	})
	require.NoError(t, err)
	return bs, bs.CharacterID, other
}

// Each character owns its items, weapons, proficiencies, bard skills and
// spellbook: the seeds' starting set is the seed character's alone, a new
// character has nothing, and adding, using, restocking, editing or removing
// one character's item or weapon, or resting, never changes the other's,
// even under the same id.
func TestCharacters_EachOwnsItsEquipmentSkillsAndSpellbook(t *testing.T) {
	ctx := context.Background()
	bs, seeded, other := twoCharacters(t)

	mine := sheetOf(t, bs, seeded)
	require.NotEmpty(t, mine.Items)
	require.NotEmpty(t, mine.Weapons)
	require.NotEmpty(t, mine.Skills)
	require.NotEmpty(t, mine.Spellbook)
	require.NotEmpty(t, mine.Mine)
	assert.Equal(t, sheet{
		Items: []itemsvc.Item{}, Weapons: []combatsvc.Weapon{}, Skills: []string{},
		Mine: []string{},
	}, sheetOf(t, bs, other), "a new character starts with nothing")

	// The other character's play, under ids the seeded character also holds.
	flask, err := bs.Items.Create(ctx, other, itemsvc.ItemInput{
		Name: localized.Of("Fire Flask"), Type: "flask", Behaviour: itemsvc.BehaviourStock, Count: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "fire_flask", flask.Item.ID, "the seeded character's id, for another character")
	_, err = bs.Items.UseItem(ctx, other, "fire_flask")
	require.NoError(t, err)
	_, err = bs.Items.Restock(ctx, other, "fire_flask", 3)
	require.NoError(t, err)
	_, err = bs.Items.Update(ctx, other, "fire_flask", itemsvc.ItemPatch{Notes: new("theirs")})
	require.NoError(t, err)
	sword, err := bs.Weapons.Create(ctx, other, weaponsvc.WeaponInput{
		Name: localized.Of("Longsword"), DamageSM: "1d8", DamageL: "1d12", Kind: "melee",
	})
	require.NoError(t, err)
	require.Equal(t, "longsword", sword.Weapon.ID)
	_, err = bs.Weapons.Update(ctx, other, "longsword", weaponsvc.WeaponPatch{MagicToHit: new(3)})
	require.NoError(t, err)
	_, err = bs.Weapons.Delete(ctx, other, "longsword")
	require.NoError(t, err)
	_, err = bs.Rest.LongRest(ctx, other)
	require.NoError(t, err)
	assert.Equal(t, mine, sheetOf(t, bs, seeded), "the other character's play leaves the seeded one's sheet")

	// What only the seeded character owns is unknown to the other.
	_, err = bs.Items.UseItem(ctx, other, "arrows_plus1")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
	_, err = bs.Weapons.Update(ctx, other, "dagger", weaponsvc.WeaponPatch{MagicToHit: new(1)})
	require.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
	_, err = bs.Combat.ResolveAttack(ctx, other, combatsvc.AttackRequest{WeaponID: "dagger"})
	require.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)
	_, err = bs.Skills.Roll(ctx, other, "tumbling", skillsvc.Adjustment{})
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = bs.Prep.Prepare(ctx, other, []string{"alarm"})
	require.ErrorIs(t, err, prepsvc.ErrNotInSpellbook)

	// The seeded character's play leaves the other's.
	theirs := sheetOf(t, bs, other)
	_, err = bs.Items.UseItem(ctx, seeded, "fire_flask")
	require.NoError(t, err)
	_, err = bs.Items.Restock(ctx, seeded, "arrows_plus1", 2)
	require.NoError(t, err)
	_, err = bs.Weapons.Update(ctx, seeded, "dagger", weaponsvc.WeaponPatch{MagicToHit: new(2)})
	require.NoError(t, err)
	_, err = bs.Items.Delete(ctx, seeded, "wand_lightning")
	require.NoError(t, err)
	assert.Equal(t, theirs, sheetOf(t, bs, other), "the seeded character's play leaves the other's sheet")
}

// Undoing an item use or a weapon edit restores the row of the character
// whose entry it pops, and only that row, though the other character holds
// the same id.
func TestCharacters_UndoRestoresTheEntrysCharactersRow(t *testing.T) {
	ctx := context.Background()
	bs, seeded, other := twoCharacters(t)
	_, err := bs.Items.Create(ctx, other, itemsvc.ItemInput{
		Name: localized.Of("Fire Flask"), Type: "flask", Behaviour: itemsvc.BehaviourStock, Count: 7,
	})
	require.NoError(t, err)
	_, err = bs.Weapons.Create(ctx, other, weaponsvc.WeaponInput{
		Name: localized.Of("Dagger"), DamageSM: "1d4", DamageL: "1d3", Kind: "melee",
	})
	require.NoError(t, err)

	count := func(characterID int64) int {
		items, err := bs.Items.List(ctx, characterID)
		require.NoError(t, err)
		for _, it := range items {
			if it.ID == "fire_flask" {
				return it.Count
			}
		}
		t.Fatalf("character %d has no fire_flask", characterID)
		return 0
	}
	toHit := func(characterID int64) int {
		weapons, err := bs.Combat.ListWeapons(ctx, characterID)
		require.NoError(t, err)
		for _, w := range weapons {
			if w.ID == "dagger" {
				return w.MagicToHit
			}
		}
		t.Fatalf("character %d has no dagger", characterID)
		return 0
	}
	flasks, daggerToHit := count(seeded), toHit(seeded)

	// The seeded character's use, then the other's: each undo pops its own.
	used, err := bs.Items.UseItem(ctx, seeded, "fire_flask")
	require.NoError(t, err)
	theirs, err := bs.Items.UseItem(ctx, other, "fire_flask")
	require.NoError(t, err)
	require.Equal(t, flasks-1, count(seeded))
	require.Equal(t, 6, count(other))
	_, err = bs.Log.Undo(ctx, seeded, used.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, flasks, count(seeded), "the seeded character's flask is back")
	assert.Equal(t, 6, count(other), "the other's use stands")
	_, err = bs.Log.Undo(ctx, other, theirs.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, 7, count(other))
	assert.Equal(t, flasks, count(seeded))

	edit, err := bs.Weapons.Update(ctx, other, "dagger", weaponsvc.WeaponPatch{MagicToHit: new(4)})
	require.NoError(t, err)
	_, err = bs.Weapons.Update(ctx, seeded, "dagger", weaponsvc.WeaponPatch{MagicToHit: new(daggerToHit + 1)})
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, other, edit.LogEntryID)
	require.NoError(t, err)
	assert.Zero(t, toHit(other), "the other's dagger is restored")
	assert.Equal(t, daggerToHit+1, toHit(seeded), "the seeded character's edit stands")
}

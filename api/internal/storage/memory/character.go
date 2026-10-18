package memory

import (
	"context"
	"maps"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Characters is the character port: the sheet reads (charactersvc.Reader),
// the details edit (detailssvc.DAL), the maximum HP (hpsvc.DAL) and the
// single-character mode's id.
type Characters struct{ s *Store }

var (
	_ charactersvc.Reader = Characters{}
	_ detailssvc.DAL      = Characters{}
	_ hpsvc.DAL           = Characters{}
)

// Characters returns the character port.
func (s *Store) Characters() Characters { return Characters{s} }

// GetCharacter returns the character, or domain.ErrCharacterNotFound.
func (c Characters) GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error) {
	var out *charactersvc.Character
	err := c.s.read(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch = clone(ch)
		out = &ch
		return nil
	})
	return out, err
}

// FirstID returns the lowest character id, or domain.ErrCharacterNotFound
// when there is none.
func (c Characters) FirstID(ctx context.Context) (int64, error) {
	var first int64
	err := c.s.read(ctx, func(t *tables) error {
		if len(t.characters) == 0 {
			return domain.ErrCharacterNotFound
		}
		ids := slices.Sorted(maps.Keys(t.characters))
		first = ids[0]
		return nil
	})
	return first, err
}

// GetDetails returns the editable details, or domain.ErrCharacterNotFound.
func (c Characters) GetDetails(ctx context.Context, id int64) (charactersvc.Details, error) {
	ch, err := c.GetCharacter(ctx, id)
	if err != nil {
		return charactersvc.Details{}, err
	}
	return ch.Details, nil
}

// UpdateDetails overwrites every editable detail of the character, or
// returns domain.ErrCharacterNotFound.
func (c Characters) UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.Details = clone(v)
		if ch.Details.Languages == nil {
			ch.Details.Languages = []string{}
		}
		t.characters[id] = ch
		return nil
	})
}

// GetMaxHP returns the character's maximum HP, or
// domain.ErrCharacterNotFound.
func (c Characters) GetMaxHP(ctx context.Context, characterID int64) (int, error) {
	ch, err := c.GetCharacter(ctx, characterID)
	if err != nil {
		return 0, err
	}
	return ch.HPMax, nil
}

// AddCharacter stores a new character from the sheet c (its id is assigned)
// with an empty spell slot list, no buffs and round 0, and returns its id. A
// character added this way owns nothing: no weapons, items, skills or
// spellbook (only the seed character gets the seed files' starting set).
func (s *Store) AddCharacter(ctx context.Context, c charactersvc.Character) (int64, error) {
	var id int64
	err := s.write(ctx, func(t *tables) error {
		id = nextID(&t.characterSeq)
		c.ID = id
		c.Details.Languages = append([]string{}, c.Details.Languages...)
		t.characters[id] = c
		t.slots[id] = []spellsvc.SpellSlot{}
		t.buffs[id] = []buffsvc.BuffEffect{}
		t.rounds[id] = roundState{}
		return nil
	})
	return id, err
}

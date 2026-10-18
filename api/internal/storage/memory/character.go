package memory

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Characters is the character port: the sheet reads (charactersvc.Reader),
// who owns and plays which character (charactersvc.Owners), the details edit
// (detailssvc.DAL), the maximum HP (hpsvc.DAL) and the realized level
// (levelupsvc.DAL).
type Characters struct{ s *Store }

var (
	_ charactersvc.Reader = Characters{}
	_ charactersvc.Owners = Characters{}
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
		// No realized level is the sheet's level, as the database's
		// realized_level NULL (ADR 0030 step 7.5).
		if ch.RealizedLevel == 0 {
			ch.RealizedLevel = ch.Sheet.Level
		}
		out = &ch
		return nil
	})
	return out, err
}

// SetRealizedLevel implements levelupsvc.DAL (ADR 0030 step 7.3), or
// returns domain.ErrCharacterNotFound.
func (c Characters) SetRealizedLevel(ctx context.Context, id int64, realizedLevel, hpMax int) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.RealizedLevel, ch.HPMax = realizedLevel, hpMax
		t.characters[id] = ch
		return nil
	})
}

// SetSkillPoints implements skillsvc.Points (ADR 0030 step 17), or returns
// domain.ErrCharacterNotFound. It stores a copy of points.
func (c Characters) SetSkillPoints(ctx context.Context, id int64, points map[string]int) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.SkillPoints = maps.Clone(points)
		t.characters[id] = ch
		return nil
	})
}

// CharactersOf implements charactersvc.Owners.
func (c Characters) CharactersOf(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error) {
	out := []charactersvc.Summary{}
	err := c.s.read(ctx, func(t *tables) error {
		for _, id := range slices.Sorted(maps.Keys(t.characters)) {
			if t.owners[id] != owner || owner == uuid.Nil || t.retired[id] {
				continue
			}
			ch := t.characters[id]
			out = append(out, charactersvc.Summary{ID: id, Name: ch.Name, Class: ch.Class})
		}
		return nil
	})
	return out, err
}

// SetCurrentCharacter implements charactersvc.Owners.
func (c Characters) SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error {
	return c.s.write(ctx, func(t *tables) error {
		_, known := t.users[owner]
		_, exists := t.characters[id]
		if !known || !exists || owner == uuid.Nil || t.owners[id] != owner || t.retired[id] {
			return domain.ErrCharacterNotFound
		}
		t.current[owner] = id
		return nil
	})
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

// AddCharacterOf is AddCharacter for a character owner owns (a user of the
// store).
func (s *Store) AddCharacterOf(ctx context.Context, owner uuid.UUID, c charactersvc.Character) (int64, error) {
	var id int64
	err := s.InTx(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.AddCharacter(ctx, c); err != nil {
			return err
		}
		return s.write(ctx, func(t *tables) error {
			if _, ok := t.users[owner]; !ok {
				return fmt.Errorf("%w: character of unknown user %s", errConstraint, owner)
			}
			t.owners[id] = owner
			return nil
		})
	})
	return id, err
}

// CreateCharacter implements charactersvc.Owners.
func (c Characters) CreateCharacter(ctx context.Context, owner uuid.UUID, ch charactersvc.Character) (int64, error) {
	ch.Details.Languages = []string{}
	return c.s.AddCharacterOf(ctx, owner, ch)
}

// ownedBy is the character id of owner's when owner has it and has not
// retired it, else domain.ErrCharacterNotFound.
func (t *tables) ownedBy(owner uuid.UUID, id int64) (charactersvc.Character, error) {
	ch, ok := t.characters[id]
	if !ok || owner == uuid.Nil || t.owners[id] != owner || t.retired[id] {
		return charactersvc.Character{}, domain.ErrCharacterNotFound
	}
	return ch, nil
}

// RenameCharacter implements charactersvc.Owners.
func (c Characters) RenameCharacter(ctx context.Context, owner uuid.UUID, id int64, name string) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, err := t.ownedBy(owner, id)
		if err != nil {
			return err
		}
		ch.Name = name
		t.characters[id] = ch
		return nil
	})
}

// RetireCharacter implements charactersvc.Owners (ADR 0029 step 7.3): its
// owner's lists no longer show it; nothing is deleted.
func (c Characters) RetireCharacter(ctx context.Context, owner uuid.UUID, id int64) error {
	return c.s.write(ctx, func(t *tables) error {
		if _, err := t.ownedBy(owner, id); err != nil {
			return err
		}
		t.retired[id] = true
		return nil
	})
}

// RepairCurrentCharacter implements charactersvc.Owners.
func (c Characters) RepairCurrentCharacter(ctx context.Context, owner uuid.UUID) error {
	return c.s.write(ctx, func(t *tables) error {
		if _, ok := t.users[owner]; !ok || t.currentOf(owner) != 0 {
			return nil
		}
		delete(t.current, owner)
		for _, id := range slices.Sorted(maps.Keys(t.characters)) {
			if t.owners[id] == owner && !t.retired[id] {
				t.current[owner] = id
				return nil
			}
		}
		return nil
	})
}

// CurrentCharacter implements charactersvc.Owners.
func (c Characters) CurrentCharacter(ctx context.Context, owner uuid.UUID) (int64, error) {
	var id int64
	err := c.s.read(ctx, func(t *tables) error {
		id = t.currentOf(owner)
		return nil
	})
	return id, err
}

// RetireCharacter retires the character whoever owns it (the tests' set-up;
// the app retires through Characters.RetireCharacter).
func (s *Store) RetireCharacter(ctx context.Context, id int64) error {
	return s.write(ctx, func(t *tables) error {
		if _, ok := t.characters[id]; !ok {
			return domain.ErrCharacterNotFound
		}
		t.retired[id] = true
		return nil
	})
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

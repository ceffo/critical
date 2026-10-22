package memory

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// Characters is the character port: the sheet reads (charactersvc.Reader),
// who owns and plays which character (charactersvc.Owners), the details edit
// (detailssvc.DAL), the maximum HP and PSPs (actionlogsvc.Maxima) and the realized level
// (levelupsvc.DAL).
type Characters struct{ s *Store }

var (
	_ charactersvc.Reader = Characters{}
	_ charactersvc.Owners = Characters{}
	_ detailssvc.DAL      = Characters{}
	_ actionlogsvc.Maxima = Characters{}
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
		ch.SetClasses(ch.Entries())
		out = &ch
		return nil
	})
	return out, err
}

// SetRealizedLevel implements levelupsvc.DAL (ADR 0030 step 7.3, ADR 0042
// step 7: the entry of class, "" the first active one), or returns
// domain.ErrCharacterNotFound.
func (c Characters) SetRealizedLevel(ctx context.Context, id int64, class string, realizedLevel, hpMax int) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		if !ch.SetEntryLevel(class, realizedLevel) {
			return domain.ErrCharacterNotFound
		}
		ch.HPMax = hpMax
		t.characters[id] = ch
		return nil
	})
}

// SetSkillPoints implements skillsvc.Points (ADR 0030 step 17), or returns
// domain.ErrCharacterNotFound. It stores a copy of points.
func (c Characters) SetSkillPoints(ctx context.Context, id int64, points skills.SetPoints) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.SkillPoints = points.Clone()
		t.characters[id] = ch
		return nil
	})
}

// SkillPoints implements skillsvc.Points: the class and a copy of the points of every skill set,
// or domain.ErrCharacterNotFound.
func (c Characters) SkillPoints(ctx context.Context, id int64) (string, skills.SetPoints, error) {
	var class string
	var points skills.SetPoints
	err := c.s.read(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		class, points = ch.Class, ch.SkillPoints.Clone()
		return nil
	})
	return class, points, err
}

// SetProficiencyPicks implements skillsvc.Picks (ADR 0037 step 12), or
// returns domain.ErrCharacterNotFound. It stores a copy of picks.
func (c Characters) SetProficiencyPicks(ctx context.Context, id int64, picks skills.Picks) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.Proficiencies = picks.Clone()
		t.characters[id] = ch
		return nil
	})
}

// GetACOverride implements acsvc.Overrides: nil for none, or
// domain.ErrCharacterNotFound.
func (c Characters) GetACOverride(ctx context.Context, id int64) (*acsvc.Override, error) {
	var out *acsvc.Override
	err := c.s.read(ctx, func(t *tables) error {
		if _, ok := t.characters[id]; !ok {
			return domain.ErrCharacterNotFound
		}
		if o, ok := t.acOverrides[id]; ok {
			out = &o
		}
		return nil
	})
	return out, err
}

// SetACOverride implements acsvc.Overrides: nil clears it, or
// domain.ErrCharacterNotFound.
func (c Characters) SetACOverride(ctx context.Context, id int64, o *acsvc.Override) error {
	return c.s.write(ctx, func(t *tables) error {
		if _, ok := t.characters[id]; !ok {
			return domain.ErrCharacterNotFound
		}
		if o == nil {
			delete(t.acOverrides, id)
		} else {
			t.acOverrides[id] = *o
		}
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
			out = append(out, charactersvc.Summary{ID: id, Name: ch.Name})
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

// GetDetails returns the editable details with the class and race codes, or domain.ErrCharacterNotFound.
func (c Characters) GetDetails(ctx context.Context, id int64) (charactersvc.Details, charactersvc.Kind, error) {
	ch, err := c.GetCharacter(ctx, id)
	if err != nil {
		return charactersvc.Details{}, charactersvc.Kind{}, err
	}
	return ch.Details, charactersvc.Kind{Class: ch.Class, Race: ch.Race}, nil
}

// UpdateDetails overwrites every editable detail of the character but its
// languages (SetLanguages, ADR 0037 step 18), or returns
// domain.ErrCharacterNotFound.
func (c Characters) UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		langs := ch.Details.Languages
		ch.Details = clone(v)
		ch.Details.Languages = langs
		if ch.Details.Languages == nil {
			ch.Details.Languages = []string{}
		}
		t.characters[id] = ch
		return nil
	})
}

// SetLanguages implements skillsvc.Picks: the character's languages (ADR 0037
// step 18), or domain.ErrCharacterNotFound. It stores a copy.
func (c Characters) SetLanguages(ctx context.Context, id int64, languages []string) error {
	return c.s.write(ctx, func(t *tables) error {
		ch, ok := t.characters[id]
		if !ok {
			return domain.ErrCharacterNotFound
		}
		ch.Details.Languages = append([]string{}, languages...)
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

// GetMaxPSP returns the character's maximum PSPs (ADR 0043 step 4.4), or
// domain.ErrCharacterNotFound.
func (c Characters) GetMaxPSP(ctx context.Context, characterID int64) (int, error) {
	ch, err := c.GetCharacter(ctx, characterID)
	if err != nil {
		return 0, err
	}
	return ch.PSPMax, nil
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
	ch.Details.Languages = append([]string{}, ch.Details.Languages...)
	ch.Proficiencies = ch.Proficiencies.Clone()
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

// RestoreSkillArmor implements charactersvc.Owners.
func (c Characters) RestoreSkillArmor(ctx context.Context, id int64, armor string) error {
	return c.s.write(ctx, func(t *tables) error {
		if _, ok := t.characters[id]; !ok {
			return domain.ErrCharacterNotFound
		}
		t.skillArmors[id] = armor
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

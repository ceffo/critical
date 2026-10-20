package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// Crit is the house crit table (combatsvc.CritStore).
type Crit struct{ s *Store }

var _ combatsvc.CritStore = Crit{}

// Crit returns the crit table port.
func (s *Store) Crit() Crit { return Crit{s} }

// List returns the stored rows by ascending range start; none means the
// table was never edited.
func (p Crit) List(ctx context.Context) ([]combatsvc.CritRow, error) {
	out := []combatsvc.CritRow{}
	err := p.s.read(ctx, func(t *tables) error {
		out = append(out, t.crit...)
		return nil
	})
	return out, err
}

// Replace swaps the whole table; two rows starting at the same roll are
// refused (the range start is the table's key).
func (p Crit) Replace(ctx context.Context, rows []combatsvc.CritRow) error {
	return p.s.write(ctx, func(t *tables) error {
		next := slices.Clone(rows)
		slices.SortStableFunc(next, func(a, b combatsvc.CritRow) int { return cmp.Compare(a.From, b.From) })
		for i := 1; i < len(next); i++ {
			if next[i].From == next[i-1].From {
				return errDuplicate("crit_table_rows", next[i].From)
			}
		}
		t.crit = next
		return nil
	})
}

// Forms is the attack form's last choices (combatsvc.FormStore).
type Forms struct{ s *Store }

var _ combatsvc.FormStore = Forms{}

// Forms returns the attack form port.
func (s *Store) Forms() Forms { return Forms{s} }

// Get returns the character's stored form; ok is false when none was saved.
func (p Forms) Get(ctx context.Context, characterID int64) (combatsvc.AttackForm, bool, error) {
	var out combatsvc.AttackForm
	var ok bool
	err := p.s.read(ctx, func(t *tables) error {
		out, ok = t.forms[characterID]
		out = clone(out)
		return nil
	})
	return out, ok, err
}

// Put stores the character's form.
func (p Forms) Put(ctx context.Context, characterID int64, f combatsvc.AttackForm) error {
	return p.s.write(ctx, func(t *tables) error {
		t.forms[characterID] = clone(f)
		return nil
	})
}

// XP is the XP ledger (xpsvc.DAL).
type XP struct{ s *Store }

var _ xpsvc.DAL = XP{}

// XP returns the XP ledger port.
func (s *Store) XP() XP { return XP{s} }

// stored is the entry as the ledger keeps it: the fields the table has.
func storedXP(characterID int64, e xpsvc.Entry) xpEntry {
	return xpEntry{characterID: characterID, Entry: xpsvc.Entry{
		ID: e.ID, Date: e.Date, Session: e.Session, Reason: e.Reason, Base: e.Base, Bonus: e.Bonus,
	}}
}

// List returns the character's entries, newest first.
func (p XP) List(ctx context.Context, characterID int64) ([]xpsvc.Entry, error) {
	out := []xpsvc.Entry{}
	err := p.s.read(ctx, func(t *tables) error {
		for _, e := range t.xp {
			if e.characterID == characterID {
				out = append(out, e.Entry)
			}
		}
		slices.SortFunc(out, func(a, b xpsvc.Entry) int { return cmp.Compare(b.ID, a.ID) })
		return nil
	})
	return out, err
}

// Get returns one entry, or xpsvc.ErrEntryNotFound.
func (p XP) Get(ctx context.Context, characterID, entryID int64) (xpsvc.Entry, error) {
	var out xpsvc.Entry
	err := p.s.read(ctx, func(t *tables) error {
		e, ok := t.xp[entryID]
		if !ok || e.characterID != characterID {
			return xpsvc.ErrEntryNotFound
		}
		out = e.Entry
		return nil
	})
	return out, err
}

// Insert writes a new entry and returns its id.
func (p XP) Insert(ctx context.Context, characterID int64, e xpsvc.Entry) (int64, error) {
	var id int64
	err := p.s.write(ctx, func(t *tables) error {
		id = nextID(&t.xpSeq)
		e.ID = id
		t.xp[id] = storedXP(characterID, e)
		return nil
	})
	return id, err
}

// Reinsert writes an entry back under its own id and date (an undone
// delete); an id the ledger holds is refused.
func (p XP) Reinsert(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	return p.s.write(ctx, func(t *tables) error {
		if _, taken := t.xp[e.ID]; taken {
			return errDuplicate("xp_entries", e.ID)
		}
		takeID(&t.xpSeq, e.ID)
		t.xp[e.ID] = storedXP(characterID, e)
		return nil
	})
}

// Update overwrites the session, reason, base and bonus of an entry; the
// date stays.
func (p XP) Update(ctx context.Context, characterID int64, e xpsvc.Entry) error {
	return p.s.write(ctx, func(t *tables) error {
		cur, ok := t.xp[e.ID]
		if !ok || cur.characterID != characterID {
			return xpsvc.ErrEntryNotFound
		}
		e.Date = cur.Date
		t.xp[e.ID] = storedXP(characterID, e)
		return nil
	})
}

// Delete removes an entry, or returns xpsvc.ErrEntryNotFound.
func (p XP) Delete(ctx context.Context, characterID, entryID int64) error {
	return p.s.write(ctx, func(t *tables) error {
		cur, ok := t.xp[entryID]
		if !ok || cur.characterID != characterID {
			return xpsvc.ErrEntryNotFound
		}
		delete(t.xp, entryID)
		return nil
	})
}

// Dice is the dice composer's favourites (dicesvc.DAL).
type Dice struct{ s *Store }

var _ dicesvc.DAL = Dice{}

// Dice returns the dice favourites port.
func (s *Store) Dice() Dice { return Dice{s} }

// List returns the character's favourites by position, then id.
func (p Dice) List(ctx context.Context, characterID int64) ([]dicesvc.Favourite, error) {
	out := []dicesvc.Favourite{}
	err := p.s.read(ctx, func(t *tables) error {
		rows := t.favourites(characterID)
		for _, r := range rows {
			out = append(out, dicesvc.Favourite{ID: r.id, Formula: r.formula})
		}
		return nil
	})
	return out, err
}

func (t *tables) favourites(characterID int64) []diceFavourite {
	var rows []diceFavourite
	for _, r := range t.dice {
		if r.characterID == characterID {
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b diceFavourite) int {
		return cmp.Or(cmp.Compare(a.position, b.position), cmp.Compare(a.id, b.id))
	})
	return rows
}

// Insert saves a formula after the last favourite (position 1 for the first
// one) and returns its id.
func (p Dice) Insert(ctx context.Context, characterID int64, formula string) (int64, error) {
	var id int64
	err := p.s.write(ctx, func(t *tables) error {
		var position int64 = 1
		if rows := t.favourites(characterID); len(rows) > 0 {
			position = rows[len(rows)-1].position + 1
		}
		id = nextID(&t.diceSeq)
		t.dice[id] = diceFavourite{characterID: characterID, id: id, formula: formula, position: position}
		return nil
	})
	return id, err
}

// Delete removes a favourite and reports whether it existed.
func (p Dice) Delete(ctx context.Context, characterID, id int64) (bool, error) {
	var found bool
	err := p.s.write(ctx, func(t *tables) error {
		r, ok := t.dice[id]
		found = ok && r.characterID == characterID
		if found {
			delete(t.dice, id)
		}
		return nil
	})
	return found, err
}

// SetPosition moves a favourite to position and reports whether it existed.
func (p Dice) SetPosition(ctx context.Context, characterID, id, position int64) (bool, error) {
	var found bool
	err := p.s.write(ctx, func(t *tables) error {
		r, ok := t.dice[id]
		found = ok && r.characterID == characterID
		if found {
			r.position = position
			t.dice[id] = r
		}
		return nil
	})
	return found, err
}

// Spellbooks is each character's spellbook (the store spellbookadpt's Book
// reads; the composition root wires it).
type Spellbooks struct{ s *Store }

// Spellbooks returns the spellbook port.
func (s *Store) Spellbooks() Spellbooks { return Spellbooks{s} }

// Keys returns the catalogue keys of the character's spellbook in sheet
// order.
func (p Spellbooks) Keys(ctx context.Context, characterID int64) ([]string, error) {
	out := []string{}
	err := p.s.read(ctx, func(t *tables) error {
		out = append(out, t.spellbooks[characterID]...)
		return nil
	})
	return out, err
}

// AddEntry writes the spell at the end of the character's spellbook (a
// spell already there stays where it is). The slice is copied: a rolled-back
// transaction shares the one before it.
func (p Spellbooks) AddEntry(ctx context.Context, characterID int64, spellKey string) error {
	return p.s.write(ctx, func(t *tables) error {
		if slices.Contains(t.spellbooks[characterID], spellKey) {
			return nil
		}
		t.spellbooks[characterID] = append(slices.Clone(t.spellbooks[characterID]), spellKey)
		return nil
	})
}

// RemoveEntry takes the spell out of the character's spellbook.
func (p Spellbooks) RemoveEntry(ctx context.Context, characterID int64, spellKey string) error {
	return p.s.write(ctx, func(t *tables) error {
		t.spellbooks[characterID] = slices.DeleteFunc(slices.Clone(t.spellbooks[characterID]),
			func(k string) bool { return k == spellKey })
		return nil
	})
}

// LearnFailure returns the realized level the character failed to learn the
// spell at; ok is false when there is none.
func (p Spellbooks) LearnFailure(ctx context.Context, characterID int64, spellKey string) (int, bool, error) {
	var level int
	var ok bool
	err := p.s.read(ctx, func(t *tables) error {
		level, ok = t.learnFailures[characterID][spellKey]
		return nil
	})
	return level, ok, err
}

// SetLearnFailure records (or replaces) the failure at level. The inner map
// is copied: a rolled-back transaction shares the one before it.
func (p Spellbooks) SetLearnFailure(ctx context.Context, characterID int64, spellKey string, level int) error {
	return p.s.write(ctx, func(t *tables) error {
		m := maps.Clone(t.learnFailures[characterID])
		if m == nil {
			m = map[string]int{}
		}
		m[spellKey] = level
		t.learnFailures[characterID] = m
		return nil
	})
}

// DeleteLearnFailure lifts the failure (none: no change).
func (p Spellbooks) DeleteLearnFailure(ctx context.Context, characterID int64, spellKey string) error {
	return p.s.write(ctx, func(t *tables) error {
		m := maps.Clone(t.learnFailures[characterID])
		delete(m, spellKey)
		t.learnFailures[characterID] = m
		return nil
	})
}

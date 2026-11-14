// Package classentrysvc owns switching one class entry of a character to another class of its
// group (ADR 0050 step 15): a mage becomes a specialist, a specialist a mage or another
// specialist. Only the entry's class changes; the opposed spells of the new class leave the
// current slot list in the same transaction, and one journal entry carries the class entries
// and the slot list as snapshots, so its undo gives both back (ADR 0018).
package classentrysvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The refusals of ADR 0050 step 15.1, each a conflict with its own code in internal/render.
// Nothing is written when one is returned.
var (
	// ErrNotAllowed is a switch that is not wizard to wizard with a specialist on one side: the
	// target is of another group, one of the two keeps no spellbook, neither is a specialist, it
	// is the entry's own class, or another entry already has it.
	ErrNotAllowed = fmt.Errorf("class switch not allowed: %w", domain.ErrConflict)
	// ErrMinimumsNotMet is a target whose ability minimums (ADR 0040 step 1.4) the sheet's scores
	// do not reach.
	ErrMinimumsNotMet = fmt.Errorf("new class minimums not met: %w", domain.ErrConflict)
	// ErrAlignmentNotAllowed is a target that does not admit the character's alignment (ADR 0040
	// step 1.5).
	ErrAlignmentNotAllowed = fmt.Errorf("new class does not admit the alignment: %w", domain.ErrConflict)
	// ErrMultiClassNotAllowed is a target that may not be one of a multi-class character's classes
	// (ADR 0050 step 14.3).
	ErrMultiClassNotAllowed = fmt.Errorf("class not allowed in a multi-class character: %w", domain.ErrConflict)
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Switch changes the class of the character's entry (its index in the class entries) to
	// class. A *domain.FieldError on "entry" for an entry the character does not have and on
	// "class" for an unknown or unplayable class; a refusal above otherwise.
	Switch(ctx context.Context, characterID int64, entry int, class string) (*Result, error)
}

// Result is what the switch did.
type Result struct {
	Entry    int
	From, To string
}

// DAL writes a character's class entries (dualclasssvc's, a ctx transaction joins).
type DAL interface {
	ReplaceClassEntries(ctx context.Context, id int64, entries []classes.Entry) error
}

// Characters reads the character view (ADR 0045).
type Characters interface {
	View(ctx context.Context, id int64) (charactersvc.View, error)
}

// Slots is the spell slot store (prepsvc's).
type Slots interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error
}

// Catalogue is the class definitions.
type Catalogue interface {
	Get(class string) (classes.Class, error)
}

// Log is the slice of the action log a switch is recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	DAL        DAL
	Characters Characters
	Slots      Slots
	// Book is the spellbook with each spell's schools: what tells an opposed spell.
	Book      prepsvc.Spellbook
	Catalogue Catalogue
	Log       Log
	Tx        domain.TxRunner
}

type service struct{ Deps }

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Slots == nil || d.Book == nil || d.Catalogue == nil || d.Log == nil || d.Tx == nil {
		return nil, errors.New("classentrysvc: nil dependency")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Switch is ADR 0050 step 15, in one transaction: the checks of 15.1, the entry's class changed
// and nothing else (15.2), the opposed spells of the new class out of the slot list (15.4), one
// journal entry with both snapshots (15.5). The book is not touched (15.3): its opposed spells
// are flagged when read.
func (s *service) Switch(ctx context.Context, characterID int64, entry int, class string) (*Result, error) {
	var res *Result
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.Characters.View(ctx, characterID)
		if err != nil {
			return err
		}
		before := c.StoredEntries()
		if entry < 0 || entry >= len(before) {
			return &domain.FieldError{Field: "entry", Message: "unknown_entry"}
		}
		target, err := s.Catalogue.Get(class)
		if err != nil || !target.Implemented() {
			return &domain.FieldError{Field: "class", Message: "unknown_class"}
		}
		from := before[entry].Class
		current, err := s.Catalogue.Get(from)
		if err != nil {
			return err
		}
		if err := check(c, before, entry, current, target); err != nil {
			return err
		}
		after := slices.Clone(before)
		after[entry].Class = class
		if err := classes.ValidateEntries(after); err != nil {
			return err
		}
		if err := s.DAL.ReplaceClassEntries(ctx, characterID, after); err != nil {
			return err
		}
		slotsBefore, slotsAfter, err := s.unprepare(ctx, characterID, target)
		if err != nil {
			return err
		}
		res = &Result{Entry: entry, From: from, To: class}
		return s.record(ctx, characterID, *res, before, after, slotsBefore, slotsAfter)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// check is ADR 0050 step 15.1, in the order of its refusals.
func check(c charactersvc.View, entries []classes.Entry, entry int, current, target classes.Class) error {
	_, currentBook := current.Lists(classes.KindSpellbook)
	_, targetBook := target.Lists(classes.KindSpellbook)
	_, currentSpec := current.Lists(classes.KindSchoolSpecialization)
	_, targetSpec := target.Lists(classes.KindSchoolSpecialization)
	taken := slices.ContainsFunc(entries, func(e classes.Entry) bool { return e.Class == target.ID })
	if taken || target.Group != current.Group || !currentBook || !targetBook || (!currentSpec && !targetSpec) {
		return ErrNotAllowed
	}
	scores := map[string]int{}
	for _, a := range charactersvc.Abilities {
		scores[string(a)], _ = c.Score(a)
	}
	if len(target.UnmetMinimums(scores)) > 0 {
		return ErrMinimumsNotMet
	}
	if c.Alignment != "" && !target.AlignmentAllowed(c.Alignment) {
		return ErrAlignmentNotAllowed
	}
	if len(classes.Active(entries)) > 1 && charactersvc.NotForMultiClass(target, c.Race) {
		return ErrMultiClassNotAllowed
	}
	return nil
}

// unprepare is step 15.4: the slot list without the prepared spells the new class opposes, the
// other slots (used flags included) as they were. It writes the list only when a slot left. An
// extra slot a spell granted is kept: it holds a spell only once cast.
func (s *service) unprepare(ctx context.Context, characterID int64, target classes.Class) (before, after []spellsvc.SpellSlot, err error) {
	before, err = s.Slots.GetSlots(ctx, characterID)
	if err != nil {
		return nil, nil, fmt.Errorf("get slots: %w", err)
	}
	var spec spellbook.Specialization
	if f, ok := target.Lists(classes.KindSchoolSpecialization); ok {
		spec = spellbook.Specialization{Schools: f.SchoolSpecialization.Schools, OppositionSchools: f.SchoolSpecialization.OppositionSchools}
	}
	book, err := s.Book.All(ctx, characterID)
	if err != nil {
		return nil, nil, fmt.Errorf("read spellbook: %w", err)
	}
	opposed := map[string]bool{}
	for _, e := range book {
		sp := spellbook.Spell{Level: e.Level, Wizard: e.Wizard, Schools: e.Schools}
		opposed[e.ID] = spellbook.School(spec, sp) == spellbook.StandingOpposed
	}
	after = make([]spellsvc.SpellSlot, 0, len(before))
	for _, sl := range before {
		if !sl.Extra && opposed[sl.SpellID] {
			continue
		}
		after = append(after, sl)
	}
	if len(after) == len(before) {
		return before, before, nil
	}
	if err := s.Slots.SaveSlots(ctx, characterID, after); err != nil {
		return nil, nil, fmt.Errorf("save slots: %w", err)
	}
	return before, after, nil
}

// record is the payload of a class_entry_switch entry. The UI words it. Add fields, never rename.
type record struct {
	Entry int    `json:"entry"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// record writes the entry with the class entries' and the slot list's snapshots, inside the
// caller's transaction (ADR 0018 step 1; ADR 0050 step 15.5).
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after []classes.Entry, slotsBefore, slotsAfter []spellsvc.SpellSlot) error {
	payload, err := json.Marshal(record(r))
	if err != nil {
		return fmt.Errorf("encode class_entry_switch payload: %w", err)
	}
	entries, err := dualclasssvc.EntriesSnapshot(before, after)
	if err != nil {
		return err
	}
	slots, err := prepsvc.SlotsSnapshot(slotsBefore, slotsAfter)
	if err != nil {
		return err
	}
	if _, err := s.Log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindClassEntrySwitch, Payload: payload,
		Snapshots: []actionlog.Snapshot{entries, slots},
	}); err != nil {
		return fmt.Errorf("record class_entry_switch: %w", err)
	}
	return nil
}

// Module declares no codec: the snapshots are the class entries' (dualclasssvc) and the slot
// list's (prepsvc), whose codecs write them back.
func (s *service) Module() domain.Module { return domain.Module{Name: "classentrysvc"} }

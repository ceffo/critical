package xpsvc

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Input bounds.
const (
	MaxBase       = 10_000_000
	MaxReasonLen  = 200
	MaxSessionLen = 60
)

func (s *service) Get(ctx context.Context, characterID int64) (*Ledger, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, err
	}
	entries, err := s.dal.List(ctx, characterID)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		a := EntryAmount(entries[i].Base, entries[i].Bonus)
		entries[i].BonusAmount, entries[i].Total = a.BonusAmount, a.Total
		entries[i].Split = splitAmount(c.Entries(), a.Total)
	}
	// The total and the progress are the first active entry's (its
	// starting XP plus its share of each award, ADR 0042 step 5; a
	// single-class character's whole ledger).
	entryList := c.Entries()
	first, _ := classes.Primary(entryList)
	amounts := make([]int, len(entries))
	for i := range entries {
		amounts[i] = entries[i].Total
	}
	total := charactersvc.EntryXP(first, amounts, len(classes.Active(entryList)))
	// The level is the target level the rules derive from that total, and
	// the thresholds are the class's (ADR 0030 step 3, amending ADR 0015
	// step 3); the realized level waits for the level-up step (step 7).
	progress, err := ComputeProgress(c.Class, c.Derived.TargetLevel, total, s.table)
	if err != nil {
		return nil, err
	}
	// ADR 0042 step 6: the prime requisites of every active entry's class.
	var primes []string
	for _, e := range classes.Active(entryList) {
		p, err := s.table.PrimeRequisites(e.Class)
		if err != nil {
			return nil, fmt.Errorf("xp ledger: %w", err)
		}
		primes = append(primes, p...)
	}
	return &Ledger{
		Opening:       c.XPTotal,
		Total:         total,
		BonusEligible: BonusEligible(primes, c),
		Progress:      progress,
		Entries:       entries,
	}, nil
}

// splitAmount is ADR 0042 step 5.1: a multi-class character's award goes in
// equal shares to every active entry (nil for one class, whose whole award is
// its own).
func splitAmount(entries []classes.Entry, total int) []ClassAmount {
	active := classes.Active(entries)
	if len(active) < 2 {
		return nil
	}
	out := make([]ClassAmount, 0, len(active))
	for _, e := range active {
		out = append(out, ClassAmount{Class: e.Class, Amount: charactersvc.Share(total, len(active))})
	}
	return out
}

func (s *service) Preview(ctx context.Context, characterID int64, base int, bonus bool) (Amount, error) {
	if err := validateBase(base); err != nil {
		return Amount{}, err
	}
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return Amount{}, err
	}
	a := EntryAmount(base, bonus)
	a.Split = splitAmount(c.Entries(), a.Total)
	return a, nil
}

func (s *service) Add(ctx context.Context, characterID int64, in EntryInput) (*Ledger, error) {
	e := Entry{
		Date: s.now().Format("2006-01-02"), Base: in.Base, Bonus: in.Bonus,
		Reason: strings.TrimSpace(in.Reason), Session: strings.TrimSpace(in.Session),
	}
	if err := validate(e); err != nil {
		return nil, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		id, err := s.dal.Insert(ctx, characterID, e)
		if err != nil {
			return err
		}
		e.ID = id
		return s.logChange(ctx, characterID, actionlog.KindXPAdd, id, nil, &e)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, characterID)
}

func (s *service) Update(ctx context.Context, characterID, entryID int64, p EntryPatch) (*Ledger, error) {
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, characterID, entryID)
		if err != nil {
			return err
		}
		e := old
		if p.Base != nil {
			e.Base = *p.Base
		}
		if p.Bonus != nil {
			e.Bonus = *p.Bonus
		}
		if p.Reason != nil {
			e.Reason = strings.TrimSpace(*p.Reason)
		}
		if p.Session != nil {
			e.Session = strings.TrimSpace(*p.Session)
		}
		if err := validate(e); err != nil {
			return err
		}
		if e.Session == old.Session && e.Reason == old.Reason && e.Base == old.Base && e.Bonus == old.Bonus {
			return nil // nothing changed: no write, no transaction (ADR 0018 step 1)
		}
		if err := s.dal.Update(ctx, characterID, e); err != nil {
			return err
		}
		return s.logChange(ctx, characterID, actionlog.KindXPEdit, entryID, &old, &e)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, characterID)
}

func (s *service) Delete(ctx context.Context, characterID, entryID int64) (*Ledger, error) {
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, characterID, entryID)
		if err != nil {
			return err
		}
		if err := s.dal.Delete(ctx, characterID, entryID); err != nil {
			return err
		}
		return s.logChange(ctx, characterID, actionlog.KindXPRemove, entryID, &old, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, characterID)
}

func validateBase(base int) error {
	if base < 1 || base > MaxBase {
		return domain.FieldErrorf("base", "base must be between 1 and %d XP", MaxBase)
	}
	return nil
}

func validate(e Entry) error {
	if err := validateBase(e.Base); err != nil {
		return err
	}
	if utf8.RuneCountInString(e.Reason) > MaxReasonLen {
		return domain.FieldErrorf("reason", "reason must be at most %d characters", MaxReasonLen)
	}
	if utf8.RuneCountInString(e.Session) > MaxSessionLen {
		return domain.FieldErrorf("session", "session must be at most %d characters", MaxSessionLen)
	}
	return nil
}

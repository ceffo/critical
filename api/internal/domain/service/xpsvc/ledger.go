package xpsvc

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
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
	}
	total := LedgerTotal(c.XPTotal, entries)
	progress, err := ComputeProgress(c.Level, total, s.table)
	if err != nil {
		return nil, err
	}
	return &Ledger{
		Opening:       c.XPTotal,
		Total:         total,
		BonusEligible: BonusEligible(c.Dex, c.Cha),
		Progress:      progress,
		Entries:       entries,
	}, nil
}

func (s *service) Preview(base int, bonus bool) (Amount, error) {
	if err := validateBase(base); err != nil {
		return Amount{}, err
	}
	return EntryAmount(base, bonus), nil
}

func (s *service) Add(ctx context.Context, characterID int64, in EntryInput) (*Ledger, error) {
	e := Entry{
		Date: s.now().Format("2006-01-02"), Base: in.Base, Bonus: in.Bonus,
		Reason: strings.TrimSpace(in.Reason), Session: strings.TrimSpace(in.Session),
	}
	if err := validate(e); err != nil {
		return nil, err
	}
	if _, err := s.dal.Insert(ctx, characterID, e); err != nil {
		return nil, err
	}
	return s.Get(ctx, characterID)
}

func (s *service) Update(ctx context.Context, characterID, entryID int64, p EntryPatch) (*Ledger, error) {
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.Get(ctx, characterID, entryID)
		if err != nil {
			return err
		}
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
		return s.dal.Update(ctx, characterID, e)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, characterID)
}

func (s *service) Delete(ctx context.Context, characterID, entryID int64) (*Ledger, error) {
	if err := s.dal.Delete(ctx, characterID, entryID); err != nil {
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

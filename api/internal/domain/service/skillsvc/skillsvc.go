// Package skillsvc owns skill checks (ADR 0014): the non-weapon proficiencies
// of the sheet (a d20 at or under an ability score plus a modifier, a natural
// 20 always fails) and the bard skills (a d100 at or under a percentage). The
// DM may move the target before the roll or on the latest check after it. A
// check changes no state: it is one skill_check entry of the action log, so
// popping it only removes it. The last check of each skill is read back from
// the log.
package skillsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// List returns every proficiency and bard skill with its target and the
	// last check rolled for it.
	List(ctx context.Context, characterID int64) (*Skills, error)
	// Roll checks the skill id on the server against its target moved by
	// adj (the zero Adjustment: none) and records it in the action log, in
	// one transaction. An unknown id wraps domain.ErrNotFound, an adjustment
	// out of range or a too long reason domain.ErrInvalidInput; nothing is
	// rolled then.
	Roll(ctx context.Context, characterID int64, id string, adj Adjustment) (*CheckResult, error)
	// Adjust sets the adjustment of the latest roll, which must be the skill
	// check entryID, replacing the one in effect (0 removes it), and
	// recomputes the verdict from the stored roll. It is an audited amend,
	// its own undoable step; refusals are AmendLatest's
	// (actionlog.LatestAmender) and domain.ErrInvalidInput.
	Adjust(ctx context.Context, characterID, entryID int64, adj Adjustment) (*CheckResult, error)
}

// DAL is the driven port over the skill catalog. Both lists come back in
// the order of the sheet. Every method joins the ctx transaction.
type DAL interface {
	ListProficiencies(ctx context.Context) ([]ProficiencyDef, error)
	ListBardSkills(ctx context.Context) ([]BardSkillDef, error)
}

// Characters reads the sheet's ability scores.
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Log records a check, reads the latest ones back and amends the latest
// roll (post-roll adjustment).
type Log interface {
	actionlog.Recorder
	actionlog.LatestAmender
	// ActiveEntries returns the newest non-voided entries of kind.
	ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
}

// Deps are the service's collaborators.
type Deps struct {
	DAL        DAL
	Characters Characters
	Dice       Dice
	Log        Log
	Tx         domain.TxRunner
}

type service struct {
	dal   DAL
	chars Characters
	dice  Dice
	log   Log
	tx    domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("skillsvc: nil dependency")
	}
	return &service{dal: d.DAL, chars: d.Characters, dice: d.Dice, log: d.Log, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

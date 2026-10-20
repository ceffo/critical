// Package skillsvc owns skill checks (ADR 0014) and plain ability checks
// (ADR 0028, the proficiency check's d20 with no proficiency): the non-weapon proficiencies
// of the sheet (a d20 at or under an ability score plus a modifier, a natural
// 20 always fails) and the bard and thief skills (a d100 at or under the
// percentage the character service derives, ADR 0030 steps 17 and 18). The
// DM may move the target before the roll or on the latest check after it. A
// check changes no state: it is one skill_check entry of the action log, so
// popping it only removes it. The last check of each skill is read back from
// the log. It also writes a thief's or a bard's skill points (ADR 0030 steps
// 17 and 18), a skill_points entry undone by its snapshot.
package skillsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
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
	// RollAbility rolls a plain check of ability (str … cha) on the server
	// against the score in effect and records it in the action log, in one
	// transaction (ADR 0028). Any other key wraps domain.ErrInvalidInput;
	// nothing is rolled or logged then.
	RollAbility(ctx context.Context, characterID int64, ability string) (*AbilityCheckResult, error)
	// AdjustAbility sets the adjustment of the latest roll, which must be
	// the ability check entryID, replacing the one in effect (0 removes it),
	// and recomputes the verdict from the stored roll (ADR 0028 steps 6 to
	// 8). It is an audited amend, its own undoable step; refusals are
	// AmendLatest's (actionlog.LatestAmender) and domain.ErrInvalidInput.
	AdjustAbility(ctx context.Context, characterID, entryID int64, adj Adjustment) (*AbilityCheckResult, error)
	// PreviewPoints is the skills tab as it would read if the skill points
	// were points (ADR 0030 steps 17 and 18): the percentages with their
	// terms and the budget, for the editor to show while the player types.
	// It writes and logs nothing; the points are refused as SetPoints
	// refuses them (a *domain.FieldError, or a *domain.BudgetError).
	PreviewPoints(ctx context.Context, characterID int64, points map[string]int) (*Skills, error)
	// SetPoints sets a thief's or a bard's skill points per skill, logged
	// and undoable (ADR 0030 steps 17 and 18). A *domain.FieldError for
	// points refused (or any point of a class without skill points), a
	// *domain.BudgetError for a spend that grows an overspend (ADR 0037
	// step 10); nothing is written or logged then.
	SetPoints(ctx context.Context, characterID int64, points map[string]int) (map[string]int, error)
}

// DAL is the driven port over the skill catalog: the character's own
// proficiencies, in the order of the sheet. It joins the ctx transaction.
// The bard's and the thief's skills are derived on the character (ADR 0030
// steps 17 and 18), not stored.
type DAL interface {
	ListProficiencies(ctx context.Context, characterID int64) ([]ProficiencyDef, error)
}

// Characters reads the sheet's ability scores.
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
	// PreviewSkillPoints is the character derived with draft skill points,
	// nothing stored.
	PreviewSkillPoints(ctx context.Context, id int64, points map[string]int) (*charactersvc.Character, error)
}

// Abilities gives the sheet's score in an ability and the score in effect
// with the effects the app applies to it (buffsvc, ADR 0028 step 2).
type Abilities interface {
	GetAbilityScore(ctx context.Context, characterID int64, a charactersvc.Ability) (buffsvc.AbilityScore, error)
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
	Abilities  Abilities
	Dice       Dice
	Log        Log
	Points     Points
	Tx         domain.TxRunner
}

type service struct {
	dal    DAL
	chars  Characters
	abils  Abilities
	dice   Dice
	log    Log
	points Points
	tx     domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Abilities == nil || d.Dice == nil || d.Log == nil || d.Points == nil || d.Tx == nil {
		return nil, fmt.Errorf("skillsvc: nil dependency")
	}
	return &service{dal: d.DAL, chars: d.Characters, abils: d.Abilities, dice: d.Dice, log: d.Log, points: d.Points, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

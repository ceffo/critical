// Package skilldal is the storage adapter for the skill catalog
// (skillsvc.DAL): the proficiencies and bard_skills tables. It delegates to
// bob-generated typed queries.
package skilldal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/skilldal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// DAL is the SQLite-backed implementation of skillsvc.DAL.
type DAL struct {
	q sqlite.Querier
}

var _ skillsvc.DAL = (*DAL)(nil)

// New returns a DAL bound to q.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// ListProficiencies returns the proficiencies in sheet order.
func (d *DAL) ListProficiencies(ctx context.Context) ([]skillsvc.ProficiencyDef, error) {
	rows, err := queries.ListProficiencies().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list proficiencies: %w", err)
	}
	out := make([]skillsvc.ProficiencyDef, 0, len(rows))
	for _, r := range rows {
		out = append(out, skillsvc.ProficiencyDef{ID: r.ID, Ability: skillsvc.Ability(r.Ability), Modifier: int(r.Modifier)})
	}
	return out, nil
}

// ListBardSkills returns the bard skills in sheet order.
func (d *DAL) ListBardSkills(ctx context.Context) ([]skillsvc.BardSkillDef, error) {
	rows, err := queries.ListBardSkills().All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("list bard skills: %w", err)
	}
	out := make([]skillsvc.BardSkillDef, 0, len(rows))
	for _, r := range rows {
		out = append(out, skillsvc.BardSkillDef{ID: r.ID, Percent: int(r.Percent)})
	}
	return out, nil
}

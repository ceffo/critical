// Package skilldal is the storage adapter for the skill catalog
// (skillsvc.DAL): the proficiencies and bard_skills tables. It delegates to
// sqlc-generated queries.
package skilldal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/skilldal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// DAL is the implementation of skillsvc.DAL over the sqlc-generated queries.
type DAL struct {
	q queries.Querier
}

var _ skillsvc.DAL = (*DAL)(nil)

// New returns a DAL running its queries on q. The composition root builds q
// over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) *DAL { return &DAL{q: q} }

// ListProficiencies returns the character's proficiencies in sheet order.
func (d *DAL) ListProficiencies(ctx context.Context, characterID int64) ([]skillsvc.ProficiencyDef, error) {
	rows, err := d.q.ListProficiencies(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list proficiencies: %w", err)
	}
	out := make([]skillsvc.ProficiencyDef, 0, len(rows))
	for _, r := range rows {
		out = append(out, skillsvc.ProficiencyDef{ID: r.ID, Ability: skillsvc.Ability(r.Ability), Modifier: int(r.Modifier)})
	}
	return out, nil
}

package skilldal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/skilldal"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_InPositionOrder(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(ctx))
	for _, q := range []string{
		`INSERT INTO proficiencies (id, position, ability, modifier) VALUES ('b', 1, 'int', -2), ('a', 0, 'dex', 3)`,
		`INSERT INTO bard_skills (id, position, percent) VALUES ('y', 1, 50), ('x', 0, 85)`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}
	dal := skilldal.New(db)

	profs, err := dal.ListProficiencies(ctx)
	require.NoError(t, err)
	assert.Equal(t, []skillsvc.ProficiencyDef{
		{ID: "a", Ability: skillsvc.AbilityDex, Modifier: 3},
		{ID: "b", Ability: skillsvc.AbilityInt, Modifier: -2},
	}, profs)
	skills, err := dal.ListBardSkills(ctx)
	require.NoError(t, err)
	assert.Equal(t, []skillsvc.BardSkillDef{{ID: "x", Percent: 85}, {ID: "y", Percent: 50}}, skills)
}

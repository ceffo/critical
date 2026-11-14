package wire_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// ADR 0050 step 8: a specialist's free spell says ofSchool; a mage's has no
// such field.
func TestGrantsDTO_Step8_TheFreeSpellSaysOfSchool(t *testing.T) {
	b, err := json.Marshal(wire.GrantsDTO(skills.LevelGrants{}, []spellbook.FreeSpell{
		{Level: 3, SpellLevel: 2, OfSchool: true}, {Level: 5, SpellLevel: 3},
	}).FreeSpells)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"level":3,"spellLevel":2,"ofSchool":true},{"level":5,"spellLevel":3}]`, string(b))
}

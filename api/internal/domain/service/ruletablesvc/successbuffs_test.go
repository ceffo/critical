package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0037 steps 20.1 and 20.6: the buff a successful check puts on is rules data keyed by
// proficiency id, and the Tumbling house rule is its one row: AC -4 for 1 round. Any other
// proficiency puts none.
func TestTumblingRule_Step20_OneRowInTheRulesData(t *testing.T) {
	assert.Equal(t, []string{"tumbling"}, ruletablesvc.SuccessBuffIDs())
	b, ok := ruletablesvc.SuccessBuffFor("tumbling")
	require.True(t, ok)
	assert.Equal(t, ruletablesvc.SuccessBuff{AC: -4, Rounds: 1}, b)
	for _, id := range []string{"jumping", "musical_instrument", ""} {
		_, ok := ruletablesvc.SuccessBuffFor(id)
		assert.False(t, ok, id)
	}
}

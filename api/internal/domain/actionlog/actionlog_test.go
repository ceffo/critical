package actionlog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// The roll kinds replace the result card and end the correction window of
// the roll before them; state changes do not.
func TestIsRoll(t *testing.T) {
	cases := []struct {
		name string
		kind actionlog.Kind
		want bool
	}{
		{"attack", actionlog.KindAttack, true},
		{"save", actionlog.KindSave, true},
		{"skill check", actionlog.KindSkillCheck, true},
		// ADR 0028 step 5.
		{"ability check", actionlog.KindAbilityCheck, true},
		{"hp change", actionlog.KindHPChange, false},
		{"buff change", actionlog.KindBuffChange, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.IsRoll(c.kind))
		})
	}
}

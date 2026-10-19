package actionlog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0011 step 2: the condition thresholds, at each boundary. Max 40: a
// quarter is 10; max 41: a quarter is 10.25.
func TestConditionOf(t *testing.T) {
	cases := []struct {
		name         string
		current, max int
		want         actionlog.Condition
	}{
		{"full HP", 40, 40, actionlog.ConditionOK},
		{"one above a quarter", 11, 40, actionlog.ConditionOK},
		{"exactly a quarter", 10, 40, actionlog.ConditionBadlyWounded},
		{"just above a fractional quarter", 11, 41, actionlog.ConditionOK},
		{"just below a fractional quarter", 10, 41, actionlog.ConditionBadlyWounded},
		{"1 HP", 1, 40, actionlog.ConditionBadlyWounded},
		{"1 HP of 3 is above a quarter", 1, 3, actionlog.ConditionOK},
		{"exactly 0", 0, 40, actionlog.ConditionUnconscious},
		{"-1", -1, 40, actionlog.ConditionUnconscious},
		{"-9, one above the floor", -9, 40, actionlog.ConditionUnconscious},
		{"-10, the floor", -10, 40, actionlog.ConditionDead},
		{"below the floor (legacy)", -12, 40, actionlog.ConditionDead},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.ConditionOf(c.current, c.max))
		})
	}
}

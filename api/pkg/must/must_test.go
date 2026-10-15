package must_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/pkg/must"
)

func TestDo_ReturnsValue(t *testing.T) {
	assert.Equal(t, 7, must.Do(7, nil))
}

func TestDo_PanicsOnError(t *testing.T) {
	boom := errors.New("boom")
	assert.PanicsWithError(t, "boom", func() { must.Do(1, boom) })
}

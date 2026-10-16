package memory

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type cloned struct {
	N       *int
	S       []string
	M       map[string][]int
	Raw     json.RawMessage
	Any     any
	At      time.Time
	private []int
}

// clone shares no memory with its source: changing the copy's pointers,
// slices, maps and interface values leaves the source as it was.
func TestClone_SharesNoMemory(t *testing.T) {
	n := 1
	src := cloned{
		N: &n, S: []string{"a"}, M: map[string][]int{"k": {1}}, Raw: json.RawMessage(`{}`),
		Any: &n, At: time.Unix(5, 0).UTC(), private: []int{1},
	}
	c := clone(src)
	assert.Equal(t, src, c)
	*c.N = 2
	c.S[0] = "b"
	c.M["k"][0] = 2
	c.Raw[0] = '['
	*c.Any.(*int) = 3
	assert.Equal(t, 1, n)
	assert.Equal(t, []string{"a"}, src.S)
	assert.Equal(t, []int{1}, src.M["k"])
	assert.Equal(t, `{}`, string(src.Raw))
	var nilSlice []int
	assert.Nil(t, clone(nilSlice), "nil stays nil")
	assert.NotNil(t, clone([]int{}), "empty stays empty")
}

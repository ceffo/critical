package composition_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0030 step 9 through the class's features (ADR 0040 step 3): the draft
// says whether the form offers the 18/xx field, so the client has no rule of
// its own. It is offered at Strength 18 to a class that lists
// exceptional_strength, of a race that rolls it (not a halfling), and only
// once the Strength is entered.
func TestDraft_Step9_TheServerOffersExceptionalStrength(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"a human fighter of Str 18", `{"class":"fighter","raceCode":"human","str":18}`, true},
		{"a human fighter of Str 17", `{"class":"fighter","raceCode":"human","str":17}`, false},
		{"a human mage of Str 18", `{"class":"mage","raceCode":"human","str":18}`, false},
		{"a halfling fighter of Str 18", `{"class":"fighter","raceCode":"halfling","str":18}`, false},
		{"a fighter with no Strength yet", `{"class":"fighter","raceCode":"human"}`, false},
		{"a Strength of 18 with no class yet", `{"raceCode":"human","str":18}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				ExceptionalStrength *bool `json:"exceptionalStrength"`
			}
			require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/draft", tc.body, &got), tc.body)
			require.NotNil(t, got.ExceptionalStrength, "always sent")
			assert.Equal(t, tc.want, *got.ExceptionalStrength)
		})
	}
}

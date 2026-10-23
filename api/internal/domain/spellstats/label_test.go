package spellstats

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The labels the parser sets are the keys of the web's lookup.values.<stat>, which holds their
// wording: a key on one side only would fall back to the raw text without a failing test.
func TestLabels_AreTheWebsLookupValueKeys(t *testing.T) {
	blob, err := os.ReadFile("../../../../web/src/i18n/locales/en/common.json")
	require.NoError(t, err)
	var doc struct {
		Lookup struct {
			Values map[string]map[string]json.RawMessage `json:"values"`
		} `json:"lookup"`
	}
	require.NoError(t, json.Unmarshal(blob, &doc))

	for stat, set := range map[string]map[string]bool{
		"castingTime": castingTimeLabels, "range": rangeLabels, "duration": durationLabels,
		"area": areaLabels, "save": saveLabels,
	} {
		web := slices.Sorted(maps.Keys(doc.Lookup.Values[stat]))
		assert.Equal(t, web, slices.Sorted(maps.Keys(set)), "labels of %s", stat)
	}
	assert.Len(t, doc.Lookup.Values, 5, "a stat with values in the web has a label list here")
}

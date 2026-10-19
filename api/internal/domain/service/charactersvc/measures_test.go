package charactersvc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ADR 0032 step 8.1: a stored height names centimetres, metres, or feet and inches, en and fr; a
// bare one under 3 is metres and otherwise centimetres; anything else names nothing.
func TestParseHeight_Step8_1_EachUnit(t *testing.T) {
	for raw, want := range map[string]float64{
		"180": 180, "180 cm": 180, "180cm": 180, "182,5 cm": 182.5, "1.80": 180, "1,80": 180, "1.8 m": 180,
		"1,75 m": 175, "1m80": 180, "1 m 80": 180, "2 mètres": 200, "1 metre": 100,
		`5'11"`: 180.3, "5'11": 180.3, "5' 11\"": 180.3, "5 ft 11 in": 180.3, "5ft 11in": 180.3,
		"6 feet 2 inches": 188, "5 pieds 11 pouces": 180.3, "5'": 152.4, "6 ft": 182.9,
		"71 in": 180.3, "71 inches": 180.3, "5′11″": 180.3, "  170 CM ": 170,
	} {
		got, ok := ParseHeight(raw)
		assert.True(t, ok, "%q", raw)
		assert.InDelta(t, want, got, 0.05, "%q", raw)
	}
	for _, raw := range []string{"", "tall", "grand", "average", "5'11 or so", "6", "5.5", "3", "49", "0", "0 cm", "3000", "-10", "huge 180"} {
		_, ok := ParseHeight(raw)
		assert.False(t, ok, "%q is not a height", raw)
	}
}

// ADR 0032 step 8.1: a stored weight names kilograms or pounds; a bare one is kilograms.
func TestParseWeight_Step8_1_EachUnit(t *testing.T) {
	for raw, want := range map[string]float64{
		"75": 75, "75 kg": 75, "75kg": 75, "75,5 kg": 75.5, "70 kilos": 70, "70 kilogrammes": 70,
		"165 lb": 74.8, "165lbs": 74.8, "165 pounds": 74.8, "  80 KG ": 80,
	} {
		got, ok := ParseWeight(raw)
		assert.True(t, ok, "%q", raw)
		assert.InDelta(t, want, got, 0.05, "%q", raw)
	}
	for _, raw := range []string{"", "heavy", "lourd", "0", "0 kg", "9999 kg", "12 stone", "-5"} {
		_, ok := ParseWeight(raw)
		assert.False(t, ok, "%q is not a weight", raw)
	}
}

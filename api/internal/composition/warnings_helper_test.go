package composition_test

// warningView is a warning of GET /api/character as these tests read it.
type warningView struct {
	Code      string   `json:"code"`
	Field     string   `json:"field"`
	Count     *int     `json:"count"`
	Limit     *int     `json:"limit"`
	Value     *float64 `json:"value"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Pool      string   `json:"pool"`
	Spent     *int     `json:"spent"`
	Available *int     `json:"available"`
	Left      *int     `json:"left"`
}

// slotCodes are the slot warnings of ADR 0037 step 11: a character created without picks always
// has slots to spend, which the tests about other warnings leave out.
var slotCodes = map[string]bool{"slots_to_spend": true, "slots_over_budget": true}

// withoutSlots is ws without the slot warnings.
func withoutSlots(ws []warningView) []warningView {
	out := []warningView{}
	for _, w := range ws {
		if !slotCodes[w.Code] {
			out = append(out, w)
		}
	}
	return out
}

// withoutSlotMaps is withoutSlots on warnings decoded as maps.
func withoutSlotMaps(ws []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, w := range ws {
		if code, _ := w["code"].(string); !slotCodes[code] {
			out = append(out, w)
		}
	}
	return out
}

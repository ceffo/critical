package actionlog_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// marked is a view that tags a payload, so a test sees which ones were viewed.
func marked(p json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"viewed":` + string(p) + `}`), nil
}

func TestPayloadViews_EntryViewsThePayloadAndEveryAmendOfItsKind(t *testing.T) {
	views := actionlog.PayloadViews{actionlog.KindSave: marked}
	e := actionlog.Entry{
		Kind: actionlog.KindSave, Payload: json.RawMessage(`1`),
		Corrections: []actionlog.Correction{
			{Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`2`)},
			{Kind: actionlog.CorrectionVoid},
		},
	}

	got := views.Entry(e)

	assert.JSONEq(t, `{"viewed":1}`, string(got.Payload))
	assert.JSONEq(t, `{"viewed":2}`, string(got.Corrections[0].Payload))
	assert.Empty(t, got.Corrections[1].Payload, "an amend without a payload stays without")
	assert.JSONEq(t, `{"viewed":2}`, string(got.EffectivePayload()), "the effective payload is the viewed amend")
	assert.JSONEq(t, `1`, string(e.Payload), "the input entry is not modified")
	assert.JSONEq(t, `2`, string(e.Corrections[0].Payload), "its corrections are not shared")
}

func TestPayloadViews_EntryLeavesAKindWithoutViewAlone(t *testing.T) {
	views := actionlog.PayloadViews{actionlog.KindSave: marked}
	e := actionlog.Entry{Kind: actionlog.KindAttack, Payload: json.RawMessage(`1`)}
	assert.Equal(t, e, views.Entry(e))
	assert.Equal(t, e, actionlog.PayloadViews(nil).Entry(e))
	assert.Nil(t, views.EntryPtr(nil))
}

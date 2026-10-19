package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
)

// The service hands out entries through the payload views the registered
// modules declare, on every list-style read, so a caller cannot forget them.
func TestListReads_ServeEntriesThroughThePayloadViews(t *testing.T) {
	views := actionlog.PayloadViews{actionlog.KindSave: func(p json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"viewed":` + string(p) + `}`), nil
	}}
	dal := mocks.NewMockDAL(t)
	svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, &passTx{}, mocks.NewMockRounds(t), mocks.NewMockMaxHP(t))
	require.NoError(t, err)
	require.NoError(t, svc.Register(domain.Module{Name: "saves", Views: views}))
	stored := []actionlog.Entry{
		{ID: 2, Kind: actionlog.KindSave, Payload: json.RawMessage(`1`)},
		{ID: 1, Kind: actionlog.KindDice, Payload: json.RawMessage(`1`)},
	}
	dal.EXPECT().ListEntries(context.Background(), int64(1), 50).Return(stored, nil)
	dal.EXPECT().ListActiveEntries(context.Background(), int64(1), actionlog.KindSave, 50).Return(stored[:1], nil)

	listed, err := svc.List(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.JSONEq(t, `{"viewed":1}`, string(listed[0].Payload))
	assert.JSONEq(t, `1`, string(listed[1].Payload), "a kind without a view is served as stored")

	active, err := svc.ActiveEntries(context.Background(), 1, actionlog.KindSave, 0)
	require.NoError(t, err)
	assert.JSONEq(t, `{"viewed":1}`, string(active[0].Payload))
}

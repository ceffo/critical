package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0043 step 5 in the recording path: a PSP delta is bounded when the
// entry is written, never by rewriting the ledger. The character's maximum is
// 25; current PSPs are 25 plus the ledger total.

const pspKind = actionlog.Kind("psp_recovery")

// recordPSP records a PSP delta with the ledger total before it and returns
// the stored entry.
func recordPSP(t *testing.T, total, delta int, payload string) actionlog.NewEntry {
	t.Helper()
	svc, dal, _, tx, maxima := newSvcMaxHP(t)
	maxima.EXPECT().GetMaxPSP(mock.Anything, int64(1)).Return(25, nil).Once()
	dal.EXPECT().PSPTotal(mock.Anything, int64(1)).Return(total, nil).Once()
	var stored actionlog.NewEntry
	dal.EXPECT().InsertEntry(mock.Anything, mock.Anything, int64(0)).
		Run(func(_ context.Context, e actionlog.NewEntry, _ int64) { stored = e }).Return(42, nil).Once()
	_, err := svc.Record(context.Background(), actionlog.NewEntry{
		CharacterID: 1, Round: 2, Kind: pspKind, Summary: "rest", PSPDelta: delta, Payload: json.RawMessage(payload),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, tx.calls)
	return stored
}

// A gain is capped at the maximum (the book: never more than was lost); a
// capped entry carries pspRequested and pspApplied, an uncapped one doesn't.
func TestRecord_Step5_CapsAGainAtTheMax(t *testing.T) {
	for _, c := range []struct {
		name               string
		total, delta, want int
		wantPayload        string
	}{
		{"a gain within what was spent is stored as is", -10, 6, 6, `{}`},
		{"a gain back to the max exactly is stored as is", -10, 10, 10, `{}`},
		{"a gain past the max is capped", -10, 18, 10, `{"pspRequested":18,"pspApplied":10}`},
		{"a gain on a full pool logs 0", 0, 12, 0, `{"pspRequested":12,"pspApplied":0}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			stored := recordPSP(t, c.total, c.delta, "")
			assert.Equal(t, c.want, stored.PSPDelta)
			assert.JSONEq(t, c.wantPayload, string(stored.Payload))
		})
	}
	t.Run("the payload is kept", func(t *testing.T) {
		stored := recordPSP(t, -3, 18, `{"hours":3}`)
		assert.JSONEq(t, `{"hours":3,"pspRequested":18,"pspApplied":3}`, string(stored.Payload))
	})
}

// The total is never under 0: a loss past the PSPs left is floored there.
func TestRecord_Step5_NeverUnderZero(t *testing.T) {
	for _, c := range []struct {
		name               string
		total, delta, want int
		wantPayload        string
	}{
		{"a loss within the pool is stored as is", 0, -7, -7, `{}`},
		{"a loss to 0 exactly is stored as is", -20, -5, -5, `{}`},
		{"a loss past 0 is floored", -20, -9, -5, `{"pspRequested":-9,"pspApplied":-5}`},
		{"a loss on an empty pool logs 0", -25, -4, 0, `{"pspRequested":-4,"pspApplied":0}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			stored := recordPSP(t, c.total, c.delta, "")
			assert.Equal(t, c.want, stored.PSPDelta)
			assert.JSONEq(t, c.wantPayload, string(stored.Payload))
		})
	}
}

// An entry with no PSP delta reads neither the maximum nor the total (the
// mocks fail on an unexpected call); a failing read refuses the entry.
func TestRecord_Step5_PSPReads(t *testing.T) {
	t.Run("no delta, no read", func(t *testing.T) {
		svc, dal, _, _, _ := newSvcMaxHP(t)
		dal.EXPECT().InsertEntry(mock.Anything, mock.Anything, int64(0)).Return(42, nil).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: pspKind, Summary: "note"})
		require.NoError(t, err)
	})
	t.Run("max psp fails", func(t *testing.T) {
		svc, _, _, _, maxima := newSvcMaxHP(t)
		maxima.EXPECT().GetMaxPSP(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: pspKind, Summary: "rest", PSPDelta: 4})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("total fails", func(t *testing.T) {
		svc, dal, _, _, maxima := newSvcMaxHP(t)
		maxima.EXPECT().GetMaxPSP(mock.Anything, int64(1)).Return(25, nil).Once()
		dal.EXPECT().PSPTotal(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: pspKind, Summary: "rest", PSPDelta: 4})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("payload not an object", func(t *testing.T) {
		svc, dal, _, _, maxima := newSvcMaxHP(t)
		maxima.EXPECT().GetMaxPSP(mock.Anything, int64(1)).Return(25, nil).Once()
		dal.EXPECT().PSPTotal(mock.Anything, int64(1)).Return(0, nil).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{
			CharacterID: 1, Round: 2, Kind: pspKind, Summary: "rest", PSPDelta: 4, Payload: json.RawMessage(`[1]`),
		})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}

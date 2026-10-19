// Package snaptest reads and builds the action log's snapshot envelopes in
// tests (actionlog codec, ADR 0018 step 8), so a test states the owning
// service's state and nothing of the envelope.
package snaptest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// State is the state inside the envelope raw, as a JSON string; the test
// fails when raw is not an envelope or is absent.
func State(t testing.TB, raw json.RawMessage) string {
	t.Helper()
	_, state, err := actionlog.DecodeEnvelope(raw)
	require.NoError(t, err)
	require.NotNil(t, state, "the snapshot state is absent")
	return string(state)
}

// ID is the id inside the envelope raw.
func ID(t testing.TB, raw json.RawMessage) string {
	t.Helper()
	id, _, err := actionlog.DecodeEnvelope(raw)
	require.NoError(t, err)
	return id
}

// IsAbsent reports whether the envelope raw is absent.
func IsAbsent(t testing.TB, raw json.RawMessage) bool {
	t.Helper()
	_, state, err := actionlog.DecodeEnvelope(raw)
	require.NoError(t, err)
	return state == nil
}

// Env is the envelope of state, a JSON value, for a key without an id.
func Env(state string) json.RawMessage { return EnvID("", state) }

// EnvID is the envelope of state, a JSON value, for thing id.
func EnvID(id, state string) json.RawMessage {
	raw, err := actionlog.EncodeState(id, json.RawMessage(state))
	if err != nil {
		panic(err)
	}
	return raw
}

// Absent is the envelope of thing id, absent.
func Absent(id string) json.RawMessage {
	raw, err := actionlog.EncodeAbsent(id)
	if err != nil {
		panic(err)
	}
	return raw
}

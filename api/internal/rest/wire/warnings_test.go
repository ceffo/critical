package wire_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// ADR 0050 step 14.1: the specialist_race warning is served with its class and race, the web
// words it; the race is left out of the other warnings.
func TestWarningsDTO_Step14_TheSpecialistRaceWarningCarriesClassAndRace(t *testing.T) {
	raw, err := json.Marshal(wire.WarningsDTO([]charactersvc.Warning{
		{Code: charactersvc.CodeSpecialistRace, Field: "race", Class: "abjurer", Race: "elf"},
		{Code: charactersvc.CodeAlignmentNotAllowed, Field: "alignment"},
	}))
	require.NoError(t, err)
	assert.JSONEq(t, `[{"code":"specialist_race","field":"race","class":"abjurer","race":"elf"},`+
		`{"code":"alignment_not_allowed","field":"alignment"}]`, string(raw))
}

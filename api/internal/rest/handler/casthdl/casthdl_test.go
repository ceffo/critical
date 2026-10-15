package casthdl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
)

// The spell in a cast result carries the caster level of that cast and the
// values computed for it: the Bard's own level for a self-cast, the party
// member's for a received spell.
func TestToDTO_SpellValuesUseTheCastersLevel(t *testing.T) {
	spell := spellsvc.Spell{
		Name: "Mirror Image",
		Stats: spellstats.Parse(spellstats.Raw{
			Range: "60 yds. + 10 yds./level", Duration: "3 rds./level", SavingThrow: "None",
		}),
	}
	tests := []struct {
		name         string
		source       castsvc.Source
		level        int
		wantDuration float64
		wantRange    float64
	}{
		{"self-cast at the character's level", castsvc.SourceCast, 15, 45, 210},
		{"received from a level 9 caster", castsvc.SourceReceived, 9, 27, 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := casthdl.ToDTO(castsvc.CastResult{Source: tt.source, CasterLevel: tt.level, Spell: spell})
			assert.Equal(t, tt.level, dto.CasterLevel)
			require.NotNil(t, dto.Spell)
			assert.Equal(t, tt.level, dto.Spell.CasterLevel)
			require.NotNil(t, dto.Spell.Values)
			assert.Equal(t, tt.wantDuration, dto.Spell.Values.Duration.Amount)
			assert.Equal(t, tt.wantRange, dto.Spell.Values.Range.Amount)
		})
	}
}

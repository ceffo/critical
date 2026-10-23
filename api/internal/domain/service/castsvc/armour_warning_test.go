package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0042 step 16.2 (Q15: a warning on the cast): "an entry whose class
// allows no body armour (`armour: none`) and has a `spell_progression` warns
// `spell_in_armour` on every cast of its spells while body armour is worn,
// except an elf in elven chain." A fighter/mage in chain mail (the fighter
// allows it, so the piece itself is not warned) is warned and still casts; an
// elf fighter/mage in elven chain is not, a human one is; unarmoured, nothing.
func TestCast_Step16_2_WizardSpellInArmourWarns(t *testing.T) {
	fighterMage := func(race string) charactersvc.View {
		v := multiOf(active("fighter", "warrior", 7, false), active("mage", "wizard", 6, true))
		v.Race, v.Str = race, 17
		return v
	}
	body := func(column string) itemsvc.Item {
		return itemsvc.Item{ID: "a", Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn, SkillColumn: column}
	}
	tests := []struct {
		name string
		race string
		worn []itemsvc.Item
		want []string
	}{
		{"half-elf in chain mail", "half_elf", []itemsvc.Item{body("metal")}, []string{itemsvc.WarnSpellInArmour}},
		{"elf in elven chain", "elf", []itemsvc.Item{body("elven_chain")}, nil},
		{"human in elven chain", "human", []itemsvc.Item{body("elven_chain")}, []string{itemsvc.WarnSpellInArmour}},
		{"elf in chain mail", "elf", []itemsvc.Item{body("metal")}, []string{itemsvc.WarnSpellInArmour}},
		{"no armour", "half_elf", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), fighterMage(tt.race), 4)
			f.worn = tt.worn
			f.prepared(slot("stoneskin", "Stoneskin", 4))
			r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err, "a warning never refuses the cast")
			assert.Equal(t, tt.want, r.Warnings)
			require.Len(t, f.added, 1, "the spell still applies")
		})
	}
}

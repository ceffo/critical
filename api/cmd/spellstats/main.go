// Command spellstats prints the typed stats the API parses from a spell
// catalogue artifact (ADR 0021 step 6), as JSON: one object per spell with
// its id, its six raw stat texts and their typed form. The web's stat
// coverage report reads it (`just stat-coverage`).
//
// Usage: spellstats <catalogue.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

type spellOut struct {
	ID           string           `json:"id"`
	CastingTime  string           `json:"casting_time"`
	Range        string           `json:"range"`
	Duration     string           `json:"duration"`
	AreaOfEffect string           `json:"area_of_effect"`
	Components   string           `json:"components"`
	SavingThrow  string           `json:"saving_throw"`
	Stats        spellstats.Stats `json:"stats"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: spellstats <catalogue.json>")
		os.Exit(2)
	}
	cat, err := catalogueadpt.Load(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := make([]spellOut, 0, len(cat.Spells))
	for _, s := range cat.Spells {
		out = append(out, spellOut{
			ID: s.ID, CastingTime: s.CastingTime, Range: s.Range, Duration: s.Duration,
			AreaOfEffect: s.AreaOfEffect, Components: s.Components, SavingThrow: s.SavingThrow,
			Stats: spellstats.Parse(spellstats.Raw{
				CastingTime: s.CastingTime, Range: s.Range, Duration: s.Duration,
				AreaOfEffect: s.AreaOfEffect, Components: s.Components, SavingThrow: s.SavingThrow,
			}),
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

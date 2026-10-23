// Package viewtest builds a charactersvc.View for a service's unit test with
// no derivation (ADR 0045 step 6.1). The builder holds no rule: it never
// computes a THAC0, a save or a slot, so a test states the figure it wants on
// the returned literal. Tests only.
package viewtest

import (
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Option adjusts the view being built.
type Option func(*charactersvc.View)

// Single is a character with one active entry of class at level.
func Single(class string, level int, opts ...Option) charactersvc.View {
	return Multi(append([]Option{WithEntry(class, level)}, opts...)...)
}

// Multi is a character with the entries the options add, in order (WithEntry,
// WithFrozen), and the other figures they set.
func Multi(opts ...Option) charactersvc.View {
	v := charactersvc.View{ID: 1, Name: "Test"}
	for _, o := range opts {
		o(&v)
	}
	return v
}

// WithEntry adds an active entry of class at level (its target level too).
func WithEntry(class string, level int) Option {
	return func(v *charactersvc.View) {
		v.Entries = append(v.Entries, charactersvc.EntryView{
			Class: class, State: classes.EntryActive, RealizedLevel: level, TargetLevel: level,
		})
	}
}

// WithFrozen adds a dual-class's old entry of class, left at level.
func WithFrozen(class string, leftAt int) Option {
	return func(v *charactersvc.View) {
		v.Entries = append(v.Entries, charactersvc.EntryView{
			Class: class, State: classes.EntryFrozen, RealizedLevel: leftAt, LeftAtLevel: leftAt,
		})
	}
}

// WithIdentity sets the id, name and race.
func WithIdentity(id int64, name, race string) Option {
	return func(v *charactersvc.View) { v.ID, v.Name, v.Race = id, name, race }
}

// WithScores sets the six ability scores.
func WithScores(str, dex, con, intelligence, wis, cha int) Option {
	return func(v *charactersvc.View) {
		v.Str, v.Dex, v.Con, v.Int, v.Wis, v.Cha = str, dex, con, intelligence, wis, cha
	}
}

// WithPSP sets the PSP pool (current and maximum) and the stored maximum.
func WithPSP(current, max int) Option {
	return func(v *charactersvc.View) {
		v.PSPMax = max
		v.PSP = &charactersvc.PSPPool{Current: current, Max: max}
	}
}

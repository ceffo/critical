package actionlog

import (
	"context"
	"slices"
	"sync"
)

// State-key families that no service snapshots, named by the log itself for
// the changes a request reports (ADR 0018 step 11): FamilyLog when an entry
// was recorded, popped, replayed or amended (the journal changed), FamilyHP
// when one of them moves current HP (the ledger is the log's HP deltas). No
// module may claim them.
const (
	FamilyLog = "log"
	FamilyHP  = "hp"
)

// IsLogFamily reports whether family is one the log names itself.
func IsLogFamily(family string) bool {
	return family == FamilyLog || family == FamilyHP
}

// Changes collects the state keys of every action-log transaction a request
// commits (ADR 0018 step 11): the REST layer puts one in the request's
// context and answers with its keys, so the web refetches what they name and
// nothing else. A transaction that rolls back adds nothing. Safe for
// concurrent use.
type Changes struct {
	mu   sync.Mutex
	keys map[string]bool
}

type changesKey struct{}

// WithChanges returns ctx carrying a new, empty Changes, and that Changes.
func WithChanges(ctx context.Context) (context.Context, *Changes) {
	c := &Changes{keys: map[string]bool{}}
	return context.WithValue(ctx, changesKey{}, c), c
}

// ChangesFrom returns the Changes ctx carries, nil when it carries none (a
// write outside a request, a test).
func ChangesFrom(ctx context.Context) *Changes {
	c, _ := ctx.Value(changesKey{}).(*Changes)
	return c
}

// Add records keys as changed. A nil Changes ignores them.
func (c *Changes) Add(keys ...string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		c.keys[k] = true
	}
}

// Keys returns the keys recorded so far, sorted, each once.
func (c *Changes) Keys() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.keys))
	for k := range c.keys {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TouchedKeys are the state keys a write of e changes, by e's recording, its
// pop or its replay: FamilyLog, FamilyHP when e moves current HP (its delta,
// or an amend's), and the key of each of its snapshots.
func TouchedKeys(e Entry) []string {
	keys := []string{FamilyLog}
	if movesHP(e.HPDelta, e.Corrections) {
		keys = append(keys, FamilyHP)
	}
	for _, s := range e.Snapshots {
		keys = append(keys, s.Key)
	}
	return keys
}

// NewEntryKeys is TouchedKeys for an entry being recorded.
func NewEntryKeys(e NewEntry) []string {
	keys := []string{FamilyLog}
	if e.HPDelta != 0 {
		keys = append(keys, FamilyHP)
	}
	for _, s := range e.Snapshots {
		keys = append(keys, s.Key)
	}
	return keys
}

func movesHP(delta int, corrections []Correction) bool {
	if delta != 0 {
		return true
	}
	for _, c := range corrections {
		if c.HPDelta != nil && *c.HPDelta != 0 {
			return true
		}
	}
	return false
}

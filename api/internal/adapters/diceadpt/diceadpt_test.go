package diceadpt

import (
	"slices"
	"sync"
	"testing"
)

func rolls(r *Roller, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = r.RollD20()
	}
	return out
}

// ADR 0025 step 8.5: one seed is one sequence, whoever asks and whenever.
func TestNewSeeded_Step8_5_SameSeedSameSequence(t *testing.T) {
	a, b := rolls(NewSeeded(7), 200), rolls(NewSeeded(7), 200)
	if !slices.Equal(a, b) {
		t.Fatalf("same seed, different sequences:\n%v\n%v", a, b)
	}
	if slices.Equal(a, rolls(NewSeeded(8), 200)) {
		t.Fatal("different seeds gave the same sequence")
	}
}

// The seeded roller keeps the die's range for every entry point.
func TestNewSeeded_Step8_5_StaysInRange(t *testing.T) {
	r := NewSeeded(1)
	seen := map[int]bool{}
	for range 2000 {
		d := r.RollDie(6)
		if d < 1 || d > 6 {
			t.Fatalf("d6 rolled %d", d)
		}
		seen[d] = true
		if v := r.RollD100(); v < 1 || v > 100 {
			t.Fatalf("d100 rolled %d", v)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("d6 faces seen: %v", seen)
	}
	if r.RollDie(0) != 0 || r.RollDie(-3) != 0 {
		t.Fatal("a die with no sides must roll 0")
	}
}

// Concurrent requests share one roller: it must not race (go test -race).
func TestNewSeeded_Step8_5_ConcurrentUse(t *testing.T) {
	r := NewSeeded(3)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				r.RollD20()
			}
		}()
	}
	wg.Wait()
}

// The default roller stays unseeded: two of them do not repeat each other.
func TestNew_IsNotRepeatable(t *testing.T) {
	if slices.Equal(rolls(New(), 100), rolls(New(), 100)) {
		t.Fatal("the crypto roller repeated itself")
	}
}

package code

import (
	"math/rand/v2"
	"testing"
)

// TestRandomIsValid checks that every random code follows the rules: no
// out-of-range colors and no repeats.
func TestRandomIsValid(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for range 5000 { // Go note: `for range n` (Go 1.22+) repeats n times without a counter.
		c := Random(rng)
		if err := c.Validate(); err != nil {
			t.Fatalf("Random gave invalid code %v: %v", c, err)
		}
	}
}

// TestRandomIsReproducible checks that the same seed gives the same codes.
// The -seed flag depends on this to replay a game exactly.
func TestRandomIsReproducible(t *testing.T) {
	a := rand.New(rand.NewPCG(42, 99))
	b := rand.New(rand.NewPCG(42, 99))
	for i := range 100 {
		if ca, cb := Random(a), Random(b); ca != cb {
			t.Fatalf("draw %d: same seed gave %v and %v", i, ca, cb)
		}
	}
}

// TestRandomCoversEveryColorInEveryPosition is a rough fairness check: over
// many draws, every color should show up in every position. A bug such as
// only ever using the first 5 colors would fail this.
func TestRandomCoversEveryColorInEveryPosition(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	var seen [Length][NumColors]bool // Go note: a 2-D array; every element starts false.
	for range 5000 {
		c := Random(rng)
		for pos, color := range c {
			seen[pos][color] = true
		}
	}
	for pos := range Length {
		for _, color := range AllColors() {
			if !seen[pos][color] {
				t.Errorf("%v never appeared at position %d in 5000 draws", color, pos+1)
			}
		}
	}
}

// TestAllCount checks the number of possible codes. Choosing 5 different
// colors in order from 8 gives 8×7×6×5×4 = 6720 (written 8P5, "8 permute 5").
func TestAllCount(t *testing.T) {
	if got := len(All()); got != 6720 {
		t.Fatalf("len(All()) = %d, want 6720", got)
	}
}

// TestAllValidAndUnique checks that every code in All is valid and none
// appears twice.
func TestAllValidAndUnique(t *testing.T) {
	seen := make(map[Code]bool, 6720)
	for _, c := range All() {
		if err := c.Validate(); err != nil {
			t.Fatalf("All contains invalid code %v: %v", c, err)
		}
		if seen[c] {
			t.Fatalf("All contains %v twice", c)
		}
		seen[c] = true
	}
}

// TestAllOrder checks that All lists codes in a fixed order (sorted by color
// number, position by position), so anything built on it is reproducible.
func TestAllOrder(t *testing.T) {
	all := All()
	// Smallest: colors 0,1,2,3,4 = Black White Red Blue Green.
	if got := all[0].String(); got != "KWRBG" {
		t.Errorf("first code = %s, want KWRBG", got)
	}
	// Largest: colors 7,6,5,4,3 = Orange Pink Yellow Green Blue.
	if got := all[len(all)-1].String(); got != "OPYGB" {
		t.Errorf("last code = %s, want OPYGB", got)
	}
}

// TestAllReturnsCopy checks that changing the returned slice doesn't affect
// later callers. The list is built only once and shared, so a caller
// writing into it would otherwise corrupt it for everyone.
func TestAllReturnsCopy(t *testing.T) {
	first := All()
	first[0] = Code{Orange, Pink, Yellow, Green, Blue}
	if second := All(); second[0].String() != "KWRBG" {
		t.Errorf("All()[0] = %v after a caller changed its copy, want KWRBG", second[0])
	}
}

// BenchmarkAll shows that after the first call, All only pays for copying
// the slice; the list isn't rebuilt.
func BenchmarkAll(b *testing.B) {
	for b.Loop() {
		All()
	}
}

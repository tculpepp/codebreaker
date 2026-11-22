package code

import (
	"math/rand/v2"
	"slices"
	"sync"
)

// Random returns a random valid code: Length different colors in random
// order. The computer Code Maker uses it to pick its secret.
//
// The caller supplies the random number generator. Seed it the same way
// and you get the same codes, which makes games and tests reproducible.
//
// Go note: rng is passed in instead of using the package-level
// rand.IntN and friends. Those use a hidden global generator that can't be
// seeded in math/rand/v2, so results couldn't be replayed. Passing
// dependencies in like this is called "dependency injection", and it's the
// main thing that makes Go code testable.
func Random(rng *rand.Rand) Code {
	// rng.Perm(8) returns the numbers 0..7 in random order, such as
	// [5 2 7 0 3 1 6 4]. Taking the first 5 gives 5 different colors in a
	// random order, with every possible code equally likely.
	perm := rng.Perm(int(NumColors))

	var c Code
	for i := range c {
		c[i] = Color(perm[i])
	}
	return c
}

// allCodes builds the list of every valid code the first time it's called,
// then returns the same list on every later call.
//
// Go note: sync.OnceValue(f) wraps f so it runs at most once, on first use,
// and caches the result. It's safe to call from many goroutines at once.
// That's handy for data that's costly to build and might never be needed:
// a player who never faces the computer Breaker never pays for it.
var allCodes = sync.OnceValue(buildAll)

// All returns every valid code (6720 of them), sorted by color number
// position by position: KWRBG first, OPYGB last. The computer solver starts
// from this list and narrows it down after each guess.
//
// Each call returns a new copy, so callers may change or filter it in place
// without affecting anyone else.
func All() []Code {
	// Go note: slices.Clone makes a new slice with its own copy of the
	// elements. Copying 6720 five-byte codes (about 33 KB) is cheap, and it
	// means the cached list can never be corrupted by a caller.
	return slices.Clone(allCodes())
}

// buildAll generates every valid code, in sorted order, by filling one
// position at a time with each color not used yet ("backtracking").
//
// Picture it as a tree: position 1 has 8 choices, each of those leads to 7
// choices for position 2, and so on, down to 4 choices for position 5.
// Walking every branch in color order visits all 8×7×6×5×4 = 6720 codes,
// already sorted.
func buildAll() []Code {
	// Work out the total up front (8×7×6×5×4) so the slice is allocated
	// once at the right size.
	total := 1
	for i := 0; i < Length; i++ {
		total *= int(NumColors) - i
	}
	all := make([]Code, 0, total)

	var current Code

	// Go note: a function literal (closure) can use and change variables
	// from the function around it, here `all` and `current`. To make a
	// closure call itself, declare the variable first (var place func...)
	// and assign it on a separate line; `place := func(...)` would not yet
	// be in scope inside its own body.
	var place func(pos int, used uint8)
	place = func(pos int, used uint8) {
		if pos == Length {
			// All positions are filled. Appending copies the array, so
			// later changes to current don't affect the stored code.
			all = append(all, current)
			return
		}
		for color := Color(0); color < NumColors; color++ {
			// used is a bit set of colors already placed (the same trick as
			// Code.mask). Skip any color whose bit is on.
			if used&(1<<color) != 0 {
				continue
			}
			current[pos] = color
			place(pos+1, used|1<<color)
		}
	}
	place(0, 0)

	return all
}

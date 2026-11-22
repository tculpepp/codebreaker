package code

import "fmt"

// Mark is the easy-mode result for one position of a guess: whether that
// peg is the right color in the right spot, the right color in the wrong
// spot, or not in the secret at all.
//
// In the normal game the Code Maker only says how many pegs of each kind
// there are (see Feedback). In easy mode each key peg sits in the same
// position as the guess peg it scores, which tells the Breaker which pegs
// to keep.
type Mark int

// The three possible marks. MarkNone is the zero value, so an empty
// [Length]Mark means "nothing matched".
const (
	// MarkNone: this color isn't in the secret.
	MarkNone Mark = iota
	// MarkExact: right color, right spot (●).
	MarkExact
	// MarkPartial: right color, wrong spot (△).
	MarkPartial
)

// String returns "none", "exact" or "partial".
func (m Mark) String() string {
	switch m {
	case MarkNone:
		return "none"
	case MarkExact:
		return "exact"
	case MarkPartial:
		return "partial"
	default:
		return fmt.Sprintf("Mark(%d)", int(m))
	}
}

// Marks returns the feedback as marks in the Normal-difficulty order: all
// the exact marks first, then the partial ones, then none. Unlike the
// marks from the Marks function, their positions say nothing about which
// guess pegs they mean (D9). Screens draw both kinds the same way.
func (f Feedback) Marks() [Length]Mark {
	var marks [Length]Mark
	for i := range marks {
		switch {
		case i < f.Exact:
			marks[i] = MarkExact
		case i < f.Exact+f.Partial:
			marks[i] = MarkPartial
		}
	}
	return marks
}

// Marks scores a guess position by position, for easy mode. The mark at
// index i is about guess[i]. Counting the marks always gives the same
// numbers as Score.
//
// Both codes must be valid. Because colors never repeat, each guess peg
// can be judged on its own: it's exact if the secret has the same color
// in that spot, partial if the secret has that color somewhere else, and
// none otherwise. (With repeated colors this would over-count partials.)
//
// Go note: returning an array, not a slice, means the result is copied to
// the caller, needs no allocation, and can be compared with ==.
func Marks(secret, guess Code) [Length]Mark {
	var marks [Length]Mark
	inSecret := secret.mask()
	for i, c := range guess {
		switch {
		case secret[i] == c:
			marks[i] = MarkExact
		case inSecret&(1<<c) != 0:
			marks[i] = MarkPartial
		}
	}
	return marks
}

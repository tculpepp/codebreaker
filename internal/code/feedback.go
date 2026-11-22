package code

import "math/bits"

// Feedback is the result of scoring one guess: the key pegs the Code Maker
// would place on the board (requirements.md D9).
//
// Go note: a small struct of plain values like this is comparable with ==,
// so tests can write `if got != want`. Its zero value, Feedback{}, means
// "nothing matched", which is a sensible default.
type Feedback struct {
	// Exact counts pegs with the right color in the right position: black
	// key pegs, shown as ●.
	Exact int
	// Partial counts pegs with a right color in the wrong position: white
	// key pegs, shown as △.
	Partial int
}

// Solved reports whether the guess matched the secret exactly: all Length
// pegs in place (D2).
func (f Feedback) Solved() bool {
	return f.Exact == Length
}

// Score compares a guess with the secret and returns the feedback.
//
// Both codes must be valid (5 different colors, as Validate checks). The
// fast method below depends on colors never repeating; with repeats the
// counts would be wrong.
//
// How it works:
//
//  1. exact: count positions where the colors are equal.
//  2. common: count colors that appear anywhere in both codes. Each code's
//     mask has one bit per color it contains (see Code.mask). ANDing the two
//     masks keeps only the bits set in both, and bits.OnesCount8 counts them.
//  3. partial = common - exact. Every exact match is also a shared color,
//     so taking the exact matches away leaves the colors in the wrong place.
//
// Worked example, secret RGYBK vs guess RYGWP:
//
//	exact:   R=R, G≠Y, Y≠G, B≠W, K≠P                  → 1
//	masks:   secret {K,R,B,G,Y}  guess {W,R,G,Y,P}
//	AND:     {R,G,Y}                                  → common 3
//	partial: 3 - 1                                    → 2
//
// This takes a handful of CPU instructions and allocates nothing. That
// matters because the computer solver calls Score millions of times. The
// slower, simpler version it replaces is scoreSlow in feedback_test.go;
// the tests check that the two always agree.
func Score(secret, guess Code) Feedback {
	exact := 0
	for i := range secret {
		if secret[i] == guess[i] {
			exact++
		}
	}

	// Go note: & is bitwise AND; a bit is 1 in the result only when it's 1
	// in both inputs. bits.OnesCount8 counts the 1 bits in a uint8. On
	// most CPUs it compiles to a single instruction (POPCNT).
	common := bits.OnesCount8(secret.mask() & guess.mask())

	return Feedback{Exact: exact, Partial: common - exact}
}

package code

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// mustParse is a test helper that turns "RGYBK" into a Code, or stops the
// test if the string is invalid. It keeps the test tables short.
//
// Go note: t.Helper() marks this as a helper, so a failure is reported at
// the caller's line in the test table rather than inside this function.
// testing.TB is an interface shared by *testing.T and *testing.B, so the
// helper works in tests and benchmarks alike.
func mustParse(t testing.TB, s string) Code {
	t.Helper()
	c, err := Parse(s)
	if err != nil {
		t.Fatalf("mustParse(%q): %v", s, err)
	}
	return c
}

// TestScoreExamples checks hand-worked cases. Each comment shows the
// reasoning, so you can verify the expected answer yourself.
func TestScoreExamples(t *testing.T) {
	tests := []struct {
		name          string
		secret, guess string
		want          Feedback
	}{
		// Every peg matches in place.
		{"solved", "RGYBK", "RGYBK", Feedback{Exact: 5, Partial: 0}},
		// Same five colors, each shifted one place: all present, none in place.
		{"all shifted", "RGYBK", "KRGYB", Feedback{Exact: 0, Partial: 5}},
		// R and G in place; W, P and O aren't in the secret.
		{"two exact", "RGYBK", "RGWPO", Feedback{Exact: 2, Partial: 0}},
		// R in place. Y is in the secret but at position 3, not 2. G is in
		// the secret but at position 2, not 3. W and P are absent.
		{"mixed", "RGYBK", "RYGWP", Feedback{Exact: 1, Partial: 2}},
		// R and G are in the secret but in the wrong places; W, P, O absent.
		{"minimum overlap", "RGYBK", "WPORG", Feedback{Exact: 0, Partial: 2}},
		// Four in place, the fifth color absent from the secret.
		{"four exact", "RGYBK", "RGYBW", Feedback{Exact: 4, Partial: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(mustParse(t, tt.secret), mustParse(t, tt.guess))
			if got != tt.want {
				t.Errorf("Score(%s, %s) = %+v, want %+v", tt.secret, tt.guess, got, tt.want)
			}
		})
	}
}

// TestSolved checks that only 5 exact matches count as solving the code
// (requirements.md D2; the old "4" text was a bug).
func TestSolved(t *testing.T) {
	tests := []struct {
		fb   Feedback
		want bool
	}{
		{Feedback{Exact: 5}, true},
		{Feedback{Exact: 4}, false},
		{Feedback{Exact: 0, Partial: 5}, false},
	}
	for _, tt := range tests {
		if got := tt.fb.Solved(); got != tt.want {
			t.Errorf("%+v.Solved() = %v, want %v", tt.fb, got, tt.want)
		}
	}
}

// scoreSlow is a simple, obviously correct way to score a guess: count the
// exact matches, then count shared colors by checking every pair of pegs.
// It's a "reference model": TestScoreMatchesReference checks that the fast
// bit-mask version in Score always agrees with it.
func scoreSlow(secret, guess Code) Feedback {
	var fb Feedback
	for i := range guess {
		for j := range secret {
			if guess[i] == secret[j] {
				if i == j {
					fb.Exact++
				} else {
					fb.Partial++
				}
			}
		}
	}
	return fb
}

// randomPairs returns n random (secret, guess) pairs from a fixed seed, so
// every test run sees the same pairs and any failure can be reproduced.
//
// Go note: math/rand/v2's rand.New(rand.NewPCG(a, b)) makes a generator
// with a fixed seed. Passing a generator around, rather than calling the
// package-level functions, is what makes the game reproducible with -seed.
func randomPairs(n int) [][2]Code {
	rng := rand.New(rand.NewPCG(1, 2))
	pairs := make([][2]Code, n)
	for i := range pairs {
		pairs[i] = [2]Code{Random(rng), Random(rng)}
	}
	return pairs
}

// TestScoreMatchesReference compares Score with scoreSlow on many random
// pairs, plus every code against itself and against one fixed code.
func TestScoreMatchesReference(t *testing.T) {
	fixed := mustParse(t, "RGYBK")
	for _, c := range All() {
		if got, want := Score(c, c), scoreSlow(c, c); got != want {
			t.Fatalf("Score(%v, %v) = %+v, reference says %+v", c, c, got, want)
		}
		if got, want := Score(fixed, c), scoreSlow(fixed, c); got != want {
			t.Fatalf("Score(%v, %v) = %+v, reference says %+v", fixed, c, got, want)
		}
	}
	for _, p := range randomPairs(20000) {
		if got, want := Score(p[0], p[1]), scoreSlow(p[0], p[1]); got != want {
			t.Fatalf("Score(%v, %v) = %+v, reference says %+v", p[0], p[1], got, want)
		}
	}
}

// TestScoreProperties checks rules that must hold for every pair of valid
// codes, not just the hand-picked examples.
func TestScoreProperties(t *testing.T) {
	for _, p := range randomPairs(20000) {
		secret, guess := p[0], p[1]
		fb := Score(secret, guess)

		// Symmetry: scoring the guess against the secret or the other way
		// round gives the same answer. (Not obvious, but true when no
		// color repeats.)
		if back := Score(guess, secret); back != fb {
			t.Fatalf("Score not symmetric: (%v,%v)=%+v but reversed=%+v", secret, guess, fb, back)
		}

		// Bounds: with 5 colors each picked from 8, at most 3 of the
		// guess's colors can be missing from the secret, so at least 2
		// pegs always score. And no more than 5 can.
		if total := fb.Exact + fb.Partial; total < 2 || total > Length {
			t.Fatalf("Score(%v,%v)=%+v: exact+partial=%d, want 2..5", secret, guess, fb, total)
		}

		// 4 exact + 1 partial is impossible: if 4 pegs are in place, the
		// fifth color is either in place too or not in the secret at all.
		if fb.Exact == 4 && fb.Partial == 1 {
			t.Fatalf("Score(%v,%v) gave impossible 4 exact + 1 partial", secret, guess)
		}
	}
}

// BenchmarkScore measures Score's speed. The solver calls it millions of
// times, so it should be a few nanoseconds and allocate nothing. Run with:
//
//	go test -bench=Score -benchmem ./internal/code
//
// Go note: a benchmark function takes *testing.B. b.Loop() (Go 1.24+) runs
// the body as many times as needed to time it accurately and stops the
// compiler from optimizing the work away. Older code uses
// `for i := 0; i < b.N; i++` instead.
func BenchmarkScore(b *testing.B) {
	secret, guess := mustParse(b, "RGYBK"), mustParse(b, "RYGWP")
	for b.Loop() {
		Score(secret, guess)
	}
}

// BenchmarkScoreSlow is here for comparison, to show what the bit mask
// buys.
func BenchmarkScoreSlow(b *testing.B) {
	secret, guess := mustParse(b, "RGYBK"), mustParse(b, "RYGWP")
	for b.Loop() {
		scoreSlow(secret, guess)
	}
}

// ExampleScore shows scoring a guess.
func ExampleScore() {
	secret, _ := Parse("RGYBK")
	guess, _ := Parse("RYGWP")
	fb := Score(secret, guess)
	fmt.Printf("%d exact, %d partial\n", fb.Exact, fb.Partial)
	// Output: 1 exact, 2 partial
}

// TestMarksExamples checks easy-mode marks on hand-worked cases: each
// position gets its own result, in the same place as the guess peg.
func TestMarksExamples(t *testing.T) {
	const (
		x = MarkExact
		p = MarkPartial
		o = MarkNone
	)
	tests := []struct {
		name          string
		secret, guess string
		want          [Length]Mark
	}{
		{"solved", "RGYBK", "RGYBK", [Length]Mark{x, x, x, x, x}},
		{"all shifted", "RGYBK", "KRGYB", [Length]Mark{p, p, p, p, p}},
		// R in place; Y and G swapped; W and P absent.
		{"mixed", "RGYBK", "RYGWP", [Length]Mark{x, p, p, o, o}},
		// Only the 2nd peg is right: the mark is in the 2nd spot too.
		{"second peg exact", "RGYBK", "WGPOR", [Length]Mark{o, x, o, o, p}},
		{"minimum overlap", "RGYBK", "WPORG", [Length]Mark{o, o, o, p, p}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Marks(mustParse(t, tt.secret), mustParse(t, tt.guess))
			if got != tt.want {
				t.Errorf("Marks(%s, %s) = %v, want %v", tt.secret, tt.guess, got, tt.want)
			}
		})
	}
}

// TestMarksAgreeWithScore checks that easy-mode marks always add up to the
// normal feedback: as many exact and partial marks as Score counts.
func TestMarksAgreeWithScore(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		secret, guess := Random(rng), Random(rng)
		var got Feedback
		for _, m := range Marks(secret, guess) {
			switch m {
			case MarkExact:
				got.Exact++
			case MarkPartial:
				got.Partial++
			}
		}
		if want := Score(secret, guess); got != want {
			t.Fatalf("Marks(%s, %s) count %+v, Score says %+v", secret, guess, got, want)
		}
	}
}

// TestMarkString checks marks print as readable words in test failures.
func TestMarkString(t *testing.T) {
	for m, want := range map[Mark]string{MarkNone: "none", MarkExact: "exact", MarkPartial: "partial", Mark(9): "Mark(9)"} {
		if got := m.String(); got != want {
			t.Errorf("Mark(%d).String() = %q, want %q", int(m), got, want)
		}
	}
}

// TestFeedbackMarks checks the Normal-difficulty layout of feedback as
// marks: all exact marks first, then partial, then none, so the order says
// nothing about which pegs they mean (D9).
func TestFeedbackMarks(t *testing.T) {
	const (
		x = MarkExact
		p = MarkPartial
		o = MarkNone
	)
	tests := []struct {
		f    Feedback
		want [Length]Mark
	}{
		{Feedback{}, [Length]Mark{o, o, o, o, o}},
		{Feedback{Exact: 1, Partial: 2}, [Length]Mark{x, p, p, o, o}},
		{Feedback{Partial: 5}, [Length]Mark{p, p, p, p, p}},
		{Feedback{Exact: 5}, [Length]Mark{x, x, x, x, x}},
		{Feedback{Exact: 3, Partial: 2}, [Length]Mark{x, x, x, p, p}},
	}
	for _, tt := range tests {
		if got := tt.f.Marks(); got != tt.want {
			t.Errorf("%+v.Marks() = %v, want %v", tt.f, got, tt.want)
		}
	}
}

package solver

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
)

// seeded returns a random generator with a fixed seed, for reproducible
// tests.
func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

// solve plays one whole round between a Solver and secret, scoring each
// guess like the game does. It returns the guesses in order and stops at
// the first solve, after maxRows guesses, or on an error.
func solve(t testing.TB, s *Solver, secret code.Code, maxRows int) []code.Code {
	t.Helper()
	var guesses []code.Code
	for range maxRows {
		g, err := s.Next()
		if err != nil {
			t.Fatalf("secret %v: Next after %v: %v", secret, guesses, err)
		}
		guesses = append(guesses, g)
		fb := code.Score(secret, g)
		if fb.Solved() {
			break
		}
		s.Update(g, fb)
	}
	return guesses
}

// TestOpening checks that the first guess is the fixed opening, and that
// it's a valid code.
func TestOpening(t *testing.T) {
	g, err := New(seeded(1)).Next()
	if err != nil {
		t.Fatal(err)
	}
	if g != opening {
		t.Errorf("first guess = %v, want the opening %v", g, opening)
	}
	if err := opening.Validate(); err != nil {
		t.Errorf("opening %v is invalid: %v", opening, err)
	}
}

// TestRemainingShrinks checks that a new solver considers every code, and
// that feedback rules codes out: only codes that would have given the same
// feedback survive.
func TestRemainingShrinks(t *testing.T) {
	s := New(seeded(1))
	if got, want := s.Remaining(), len(code.All()); got != want {
		t.Fatalf("Remaining = %d, want %d", got, want)
	}
	secret, _ := code.Parse("WPOGR")
	g, _ := s.Next()
	fb := code.Score(secret, g)
	s.Update(g, fb)

	if s.Remaining() >= len(code.All()) || s.Remaining() == 0 {
		t.Errorf("Remaining after one guess = %d, want fewer than all but more than 0", s.Remaining())
	}
	// Every survivor must be a code that could still be the secret.
	for range 50 {
		c, _ := s.Next()
		if code.Score(c, g) != fb {
			t.Fatalf("candidate %v doesn't fit feedback %+v for %v", c, fb, g)
		}
	}
}

// TestSolvedFeedbackLeavesOneCandidate checks the edge case of being told
// a guess was right: only that code is left.
func TestSolvedFeedbackLeavesOneCandidate(t *testing.T) {
	s := New(seeded(1))
	g, _ := s.Next()
	s.Update(g, code.Feedback{Exact: code.Length})
	next, err := s.Next()
	if err != nil || next != g || s.Remaining() != 1 {
		t.Errorf("after a solve: Next = %v, %v with %d left; want %v, nil with 1", next, err, s.Remaining(), g)
	}
}

// TestContradictoryFeedback checks that feedback no code could give
// (4 exact and 1 partial is impossible: the fifth color has nowhere else
// to go) leaves no candidates, and Next says so instead of guessing.
func TestContradictoryFeedback(t *testing.T) {
	s := New(seeded(1))
	g, _ := s.Next()
	s.Update(g, code.Feedback{Exact: 4, Partial: 1})
	if s.Remaining() != 0 {
		t.Errorf("Remaining = %d, want 0", s.Remaining())
	}
	if _, err := s.Next(); !errors.Is(err, ErrNoCandidates) {
		t.Errorf("Next error = %v, want ErrNoCandidates", err)
	}
}

// TestSameSeedSameGuesses checks that a seeded solver is reproducible,
// which makes `-seed` replay whole games.
func TestSameSeedSameGuesses(t *testing.T) {
	secret, _ := code.Parse("OPYGB")
	a := solve(t, New(seeded(42)), secret, 12)
	b := solve(t, New(seeded(42)), secret, 12)
	if !slices.Equal(a, b) {
		t.Errorf("same seed, different guesses:\n%v\n%v", a, b)
	}
}

// TestNeverRepeats checks that the solver never offers a guess it already
// made in the round, so it can't trip the repeat rule (D14). A wrong guess
// scores 5 exact against itself, which never matches the feedback it
// actually got, so it's always ruled out.
func TestNeverRepeats(t *testing.T) {
	for seed := range uint64(200) {
		secret := code.Random(seeded(seed + 1000))
		guesses := solve(t, New(seeded(seed)), secret, 12)
		seen := map[code.Code]bool{}
		for _, g := range guesses {
			if seen[g] {
				t.Fatalf("seed %d: %v guessed twice in %v", seed, g, guesses)
			}
			seen[g] = true
		}
	}
}

// TestSolvesEveryCode is the requirement that matters most for the
// computer Code Breaker: it must crack every one of the 6720 possible codes
// within 12 rows (requirements.md "Computer Player"). It also logs the
// average and worst number of rows.
//
// With -short (make test-short), it checks a random sample of 500 codes
// instead, which runs much faster.
func TestSolvesEveryCode(t *testing.T) {
	all := code.All()
	if testing.Short() {
		rng := seeded(7)
		rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		all = all[:500]
	}

	total, worst := 0, 0
	for i, secret := range all {
		guesses := solve(t, New(seeded(uint64(i))), secret, 12)
		if last := guesses[len(guesses)-1]; last != secret {
			t.Fatalf("secret %v not solved in 12 rows: %v", secret, guesses)
		}
		total += len(guesses)
		worst = max(worst, len(guesses))
	}
	t.Logf("solved %d codes: average %.2f rows, worst %d", len(all), float64(total)/float64(len(all)), worst)
}

// BenchmarkRound measures one whole round: from a fresh solver to cracking
// a code. Most of the time goes into the first Update, which checks all
// 6720 codes.
func BenchmarkRound(b *testing.B) {
	secret, _ := code.Parse("OPYGB")
	rng := seeded(1)
	for b.Loop() {
		solve(b, New(rng), secret, 12)
	}
}

package solver

import (
	"errors"
	"math/rand/v2"

	"github.com/tculpepp/codebreaker/internal/code"
)

// ErrNoCandidates is returned by Next when no code fits all the feedback
// so far. The game computes feedback itself (D8), so in real play this
// means a bug.
var ErrNoCandidates = errors.New("solver: no code fits the feedback")

// opening is the first guess of every round. Any valid code works equally
// well as an opening, because every code is equally likely to be the
// secret. Using a fixed one makes the first row predictable to read in
// tests and transcripts.
var opening = code.Code{code.Red, code.Green, code.Yellow, code.Blue, code.Black}

// Solver works out a secret code from feedback. Use a new Solver for each
// round: call Next for a guess, play it, pass the feedback to Update, and
// repeat.
type Solver struct {
	rng        *rand.Rand
	candidates []code.Code // every code still consistent with all feedback
	updated    bool        // whether any feedback has been received yet
}

// New returns a Solver that considers every possible code and uses rng to
// choose among them. A generator with a fixed seed gives the same guesses
// every time.
func New(rng *rand.Rand) *Solver {
	// code.All returns a fresh copy each call, so this Solver can filter
	// it in place without affecting anyone else.
	return &Solver{rng: rng, candidates: code.All()}
}

// Next returns the next guess: the fixed opening before any feedback, and
// afterwards a random code that fits all the feedback so far. It returns
// ErrNoCandidates if no code fits.
//
// Next never suggests a code already guessed (unless it was the answer):
// a wrong guess would score 5 exact against itself, which doesn't match
// the feedback it got, so Update has already ruled it out.
func (s *Solver) Next() (code.Code, error) {
	if len(s.candidates) == 0 {
		return code.Code{}, ErrNoCandidates
	}
	if !s.updated {
		return opening, nil
	}
	// Go note: rng.IntN(n) returns a random int from 0 to n-1, each
	// equally likely, so this picks a random element of the slice.
	return s.candidates[s.rng.IntN(len(s.candidates))], nil
}

// Update records the feedback fb that guess received, and drops every
// candidate that would have scored differently. If a candidate were the
// secret, guess would have scored exactly fb against it; any other
// candidate can't be the secret.
func (s *Solver) Update(guess code.Code, fb code.Feedback) {
	s.updated = true

	// Go note: this is the standard way to filter a slice in place.
	// kept starts as an empty slice that shares s.candidates' memory
	// (s.candidates[:0] has length 0 but the same underlying array).
	// Appending a survivor writes it over an element that has already been
	// checked, so nothing unread is ever overwritten, and no new memory is
	// needed. The longhand version would build a new slice:
	//
	//	var kept []code.Code
	//	for _, c := range s.candidates { if ... { kept = append(kept, c) } }
	//
	// which is just as correct but allocates a fresh array every time.
	kept := s.candidates[:0]
	for _, c := range s.candidates {
		if code.Score(c, guess) == fb {
			kept = append(kept, c)
		}
	}
	s.candidates = kept
}

// Remaining returns how many codes still fit all the feedback.
func (s *Solver) Remaining() int { return len(s.candidates) }

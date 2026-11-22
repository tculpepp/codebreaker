package player

import (
	"context"
	"math/rand/v2"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/solver"
)

// Computer is a computer-controlled player. As Code Maker it picks a
// random code; as Code Breaker it uses a solver.Solver, which only ever
// learns the feedback in the round's history.
type Computer struct {
	rng    *rand.Rand
	solver *solver.Solver // the current round's solver; nil before the first guess
	seen   int            // how many history turns the solver has been told about
}

// Go note: compile-time check that *Computer can sit in a game.Seat.
var _ game.Participant = (*Computer)(nil)

// NewComputer returns a computer player that makes its choices with rng.
// Passing a generator with a fixed seed makes every game repeatable.
//
// Go note: the generator is passed in ("injected") rather than created
// here. That puts the caller in control of randomness: the app seeds it
// at start-up, and tests use a fixed seed to get the same codes every run.
func NewComputer(rng *rand.Rand) *Computer {
	return &Computer{rng: rng}
}

// MakeSecret picks a random valid code. It never fails.
func (c *Computer) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	return code.Random(c.rng), nil
}

// Guess returns the solver's next guess for this round.
//
// An empty history means a new round, so the solver starts fresh. Then
// only the turns it hasn't seen yet are passed to it, so each turn's
// feedback is used exactly once. If history is somehow shorter than what
// the solver has seen, it starts again and replays all of it.
//
// It returns solver.ErrNoCandidates if no code fits the feedback, which
// can't happen when the game computes the feedback.
func (c *Computer) Guess(ctx context.Context, info game.RoundInfo, history []game.Turn) (code.Code, error) {
	if c.solver == nil || len(history) == 0 || len(history) < c.seen {
		c.solver = solver.New(c.rng)
		c.seen = 0
	}
	// Go note: history[c.seen:] is a slice of the turns from index c.seen
	// to the end: the ones added since the last call. Slicing doesn't copy
	// anything; it's a new view onto the same elements.
	for _, t := range history[c.seen:] {
		c.solver.Update(t.Guess, t.Feedback)
	}
	c.seen = len(history)
	return c.solver.Next()
}

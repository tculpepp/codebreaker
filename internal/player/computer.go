package player

import (
	"context"
	"errors"
	"math/rand/v2"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// ErrCannotBreak is returned by Computer.Guess until the solver arrives in
// v0.4.0. The app doesn't seat the computer as Code Breaker before then.
var ErrCannotBreak = errors.New("player: the computer can't break codes yet")

// Computer is a computer-controlled player.
type Computer struct {
	rng *rand.Rand
}

// Go note: compile-time check that *Computer can sit in a game.Seat.
var _ game.Participant = (*Computer)(nil)

// NewComputer returns a computer player that makes its choices with rng.
// Passing a generator with a fixed seed makes every game repeatable.
//
// Go note: the generator is passed in ("injected") rather than created
// here. That puts the caller in control of randomness: the app seeds it
// from the clock, and tests use a fixed seed to get the same codes every
// run.
func NewComputer(rng *rand.Rand) *Computer {
	return &Computer{rng: rng}
}

// MakeSecret picks a random valid code. It never fails.
func (c *Computer) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	return code.Random(c.rng), nil
}

// Guess always returns ErrCannotBreak for now. It is here so a Computer
// satisfies game.Participant; v0.4.0 replaces it with the solver.
func (c *Computer) Guess(ctx context.Context, info game.RoundInfo, history []game.Turn) (code.Code, error) {
	return code.Code{}, ErrCannotBreak
}

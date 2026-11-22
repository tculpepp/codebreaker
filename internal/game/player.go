package game

import (
	"context"

	"github.com/tculpepp/codebreaker/internal/code"
)

// Go note: these interfaces are declared here, in the package that *uses*
// them, not in package player, which implements them. That's the usual Go
// style: "accept interfaces where you use them". game only asks for the
// methods it needs, player doesn't have to import game's interfaces to
// satisfy them, and tests can plug in fakes (see fakes_test.go).

// CodeMaker is anyone who can set a secret code: a human typing it hidden,
// or the computer picking one at random.
//
// MakeSecret should return a valid code. If it returns an error (for
// example the player quit), the match stops and passes the error on.
type CodeMaker interface {
	MakeSecret(ctx context.Context, info RoundInfo) (code.Code, error)
}

// CodeBreaker is anyone who can guess a code.
//
// Guess receives a copy of the guesses so far, with their feedback, and
// returns the next guess. It never sees the secret. It should return a
// valid code that isn't a repeat; human players check this themselves so a
// mistake doesn't cost a row (see CheckRepeat).
type CodeBreaker interface {
	Guess(ctx context.Context, info RoundInfo, history []Turn) (code.Code, error)
}

// Participant can play both roles, which every player must, because roles
// swap after each round.
//
// Go note: listing interfaces inside an interface ("embedding") combines
// their method sets. Participant has both MakeSecret and Guess.
type Participant interface {
	CodeMaker
	CodeBreaker
}

// Seat is one of the two players in a match.
type Seat struct {
	// Name is shown on screen, such as "Alice" or "Computer".
	Name string
	// Peg is the scoring-peg color identifying this player (D11).
	Peg PegColor
	// Player makes and breaks codes for this seat.
	Player Participant
	// Score is the player's position on the 30-point track.
	Score int
}

// RoundInfo describes the round being played. Players and the screen
// receive it with every event and call.
//
// Maker and Breaker are copies of the seats, scores included, as of the
// start of the round.
type RoundInfo struct {
	// Number is the 1-based round number.
	Number int
	// Mode is the kind of game being played.
	Mode Mode
	// MaxRows is the row limit (always MaxRows; included so the screen
	// doesn't need to know the constant).
	MaxRows int
	// Maker and Breaker are the seats in each role this round.
	Maker, Breaker Seat
	// Easy is Config.Easy: show the feedback by position (Turn.Marks).
	Easy bool
}

package game

import (
	"errors"
	"fmt"

	"github.com/tculpepp/codebreaker/internal/code"
)

// Go note: a "sentinel error" is a package-level error value that callers
// compare against with errors.Is(err, game.ErrRoundOver). errors.Is also
// looks through wrapped errors (made with fmt.Errorf("...: %w", err)), so
// the check still works after other code adds context.
//
// By convention sentinel errors are named Err..., and their message starts
// with the package name.

// ErrRoundOver is returned by Round.Submit after the round has ended. In
// normal play it never happens, so seeing it means a bug in the caller.
var ErrRoundOver = errors.New("game: round is over")

// ErrMatchOver is returned by Match.PlayRound after the match has ended.
var ErrMatchOver = errors.New("game: match is over")

// ErrInvalidSetup is wrapped by NewMatch when its configuration or seats
// are unusable. Check for it with errors.Is.
var ErrInvalidSetup = errors.New("game: invalid setup")

// RepeatGuessError reports a guess identical to one already made this round
// (requirements.md D14). The guess is refused and does not use a row.
type RepeatGuessError struct {
	// Guess is the repeated code.
	Guess code.Code
	// Row is the 1-based row where that code was first guessed.
	Row int
}

// Error implements the error interface.
func (e *RepeatGuessError) Error() string {
	return fmt.Sprintf("%v was already guessed in row %d", e.Guess, e.Row)
}

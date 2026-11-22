package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// ChoiceError reports a menu answer that isn't one of the listed numbers.
type ChoiceError struct {
	// Input is what the player typed.
	Input string
	// N is the number of choices; valid answers are 1 to N.
	N int
}

// Error implements the error interface.
func (e *ChoiceError) Error() string {
	return fmt.Sprintf("ui: choice %q is not a number from 1 to %d", e.Input, e.N)
}

// Friendly is implemented by errors that carry their own message for the
// player. FriendlyMessage shows that message, so other packages can make
// their errors friendly without ui knowing about them.
//
// Go note: this interface is how package player's "the two codes don't
// match" error gets a friendly message. player never imports ui; its error
// type just has a Friendly method, and that's enough to satisfy this
// interface.
type Friendly interface {
	Friendly() string
}

// ParseChoice reads a menu answer. s must be a number from 1 to n, and the
// result is its 0-based index (typing "1" gives 0), ready for indexing the
// list of choices. Anything else returns a *ChoiceError.
func ParseChoice(s string, n int) (int, error) {
	// Go note: strconv.Atoi ("ASCII to integer") converts "12" to 12. It
	// returns an error for anything that isn't a whole number, including
	// "" and "two".
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || i < 1 || i > n {
		return 0, &ChoiceError{Input: s, N: n}
	}
	return i - 1, nil
}

// FriendlyMessage turns an input mistake into a short message for the
// player, such as "Red is used twice (positions 1 and 4). Each color may
// appear only once." Errors it doesn't recognize are shown with their own
// text. It returns "" for a nil error.
//
// The errors are found with errors.As, so they're recognized even after
// other code has wrapped them with fmt.Errorf("...: %w", err).
func FriendlyMessage(err error) string {
	// Go note: errors.As looks through err and anything it wraps for an
	// error of the target's type. If it finds one, it stores it in the
	// target and returns true. The target must be a pointer to a variable
	// of that type, hence &invalid with invalid being a
	// *code.InvalidColorError.
	var (
		invalid *code.InvalidColorError
		length  *code.LengthError
		dup     *code.DuplicateColorError
		repeat  *game.RepeatGuessError
		choice  *ChoiceError
		self    Friendly
	)
	switch {
	case err == nil:
		return ""
	// Go note: errors.As also accepts a pointer to an interface variable.
	// It then finds the first error in the chain whose type has the
	// interface's methods, whatever that type is.
	case errors.As(err, &self):
		return self.Friendly()
	case errors.As(err, &invalid):
		// Go note: %q on a rune prints it in single quotes, like 'X', and
		// shows invisible characters as escapes, so odd input is visible.
		return fmt.Sprintf("%q isn't a color letter (position %d). Use %s.",
			invalid.Char, invalid.Pos, colorLetters())
	case errors.As(err, &length) && length.Got == 0:
		return fmt.Sprintf("Type %d color letters, like RGYBK.", code.Length)
	case errors.As(err, &length):
		return fmt.Sprintf("That's %d %s. A code needs exactly %d.",
			length.Got, plural(length.Got, "color"), code.Length)
	case errors.As(err, &dup):
		return fmt.Sprintf("%v is used twice (positions %d and %d). Each color may appear only once.",
			dup.Color, dup.First, dup.Second)
	case errors.As(err, &repeat):
		return fmt.Sprintf("You already tried %v in row %d. Try a different code.",
			repeat.Guess, repeat.Row)
	case errors.As(err, &choice):
		return fmt.Sprintf("Please type a number from 1 to %d.", choice.N)
	default:
		return err.Error()
	}
}

// Error shows err as a friendly message, marked with "!" so it stands out
// just above the prompt that's asked again. The board isn't redrawn.
func (r *Renderer) Error(err error) {
	r.write(" ! " + FriendlyMessage(err) + "\n")
}

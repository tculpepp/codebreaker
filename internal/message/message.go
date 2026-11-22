package message

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
)

// Friendly is implemented by errors that carry their own message for the
// player. For shows that message, so other packages can make their errors
// friendly without this package knowing about them.
//
// Go note: this interface is how package player's "the two codes don't
// match" error, and the terminal's menu errors, get friendly messages.
// They never import this package; their error types just have a Friendly
// method, and that's enough to satisfy this interface.
type Friendly interface {
	Friendly() string
}

// For turns a mistake into a short message for the player. Errors it
// doesn't recognize are shown with their own text. It returns "" for a nil
// error.
//
// The errors are found with errors.As, so they're recognized even after
// other code has wrapped them with fmt.Errorf("...: %w", err).
func For(err error) string {
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
	default:
		return err.Error()
	}
}

// colorLetters lists every color letter: "K W R B G Y P O".
func colorLetters() string {
	letters := make([]string, 0, code.NumColors)
	for _, c := range code.AllColors() {
		letters = append(letters, string(c.Abbrev()))
	}
	return strings.Join(letters, " ")
}

// plural returns word, with an "s" added unless n is 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

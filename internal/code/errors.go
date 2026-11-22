package code

import "fmt"

// Go note: in Go an error is any value with an `Error() string` method,
// because the built-in type is just:
//
//	type error interface { Error() string }
//
// A plain errors.New("...") is fine when callers only need the message.
// The custom struct types below carry details (which letter, which
// positions), so the UI can build a friendly message and tests can check
// exact values. Callers pull them out with errors.As (see errors_test.go).
//
// Go note: these types use pointer receivers, `func (e *LengthError)`, and
// are always created as pointers, `&LengthError{...}`. Then the error is
// *LengthError, the type to ask errors.As for. Using one form consistently
// matters: errors.As looking for *LengthError won't match a plain
// LengthError value.

// InvalidColorError reports a character that isn't one of the 8 color
// letters. Validation checks for it first (implementation-plan.md §7).
type InvalidColorError struct {
	// Char is the character as the player typed it. It is '?' when the
	// problem is a Color value outside the valid range rather than typed text.
	Char rune
	// Pos is the 1-based peg position where the character appeared.
	Pos int
}

// Error implements the error interface.
func (e *InvalidColorError) Error() string {
	return fmt.Sprintf("position %d: %q is not a color letter", e.Pos, e.Char)
}

// LengthError reports a code with the wrong number of colors (not
// Length). Validation checks for it second.
type LengthError struct {
	// Got is how many colors were given.
	Got int
}

// Error implements the error interface.
func (e *LengthError) Error() string {
	return fmt.Sprintf("got %d colors, want %d", e.Got, Length)
}

// DuplicateColorError reports a color used more than once, which the
// normal rules forbid (requirements.md D7). Validation checks for it last.
type DuplicateColorError struct {
	// Color is the repeated color.
	Color Color
	// First and Second are the 1-based positions of the first two copies.
	First, Second int
}

// Error implements the error interface.
func (e *DuplicateColorError) Error() string {
	// Go note: %v on e.Color calls its String method (fmt.Stringer), so
	// this prints "Red", not "2".
	return fmt.Sprintf("%v is used twice (positions %d and %d)", e.Color, e.First, e.Second)
}

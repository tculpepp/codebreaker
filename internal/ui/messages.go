package ui

import (
	"fmt"
	"strconv"
	"strings"

	"codebreaker/internal/message"
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

// Friendly returns the message shown to the player. It satisfies
// message.Friendly, which is how message.For explains a menu mistake
// without importing ui.
func (e *ChoiceError) Friendly() string {
	return fmt.Sprintf("Please type a number from 1 to %d.", e.N)
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

// Error shows err as a friendly message (from message.For), marked with "!" (and in red when
// color is on) so it stands out just above the prompt that's asked again.
// The board isn't redrawn.
func (r *Renderer) Error(err error) {
	r.write(" " + r.paint(sgrRed, "! "+message.For(err)) + "\n")
}

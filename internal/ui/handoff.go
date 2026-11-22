package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI escape codes used by Clear. Terminals treat these byte sequences as
// commands instead of text.
const (
	ansiHome            = "\x1b[H"  // move the cursor to the top left
	ansiClearScreen     = "\x1b[2J" // erase everything on screen
	ansiClearScrollback = "\x1b[3J" // erase the lines scrolled off the top
)

// isTerminal reports whether w is a terminal, as opposed to a pipe, a
// file, or a buffer in a test.
func isTerminal(w io.Writer) bool {
	// Go note: a type assertion with "comma ok" checks whether the
	// interface value w holds an *os.File. Only a file has a descriptor
	// (Fd) to ask the operating system about.
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// Clear wipes the screen, and the scrollback, so nothing from before can
// be seen or scrolled back to. When output isn't a terminal, escape codes
// would only be junk in a file, so a separator line is drawn instead.
func (r *Renderer) Clear() {
	if r.tty {
		r.write(ansiHome + ansiClearScreen + ansiClearScrollback)
		return
	}
	r.write("\n" + strings.Repeat("─", 40) + "\n")
}

// Handoff clears the screen and asks for the keyboard to be passed to
// next, the player who plays next (D10). The caller then waits for next
// to press Enter.
func (r *Renderer) Handoff(next string) {
	r.Clear()
	r.write("\n Pass the keyboard to " + next + ".\n")
}

// Redraw draws the current round's board again, with every row so far.
// It's used after Handoff, which cleared the screen.
func (r *Renderer) Redraw() {
	r.Board(r.info, r.turns)
}

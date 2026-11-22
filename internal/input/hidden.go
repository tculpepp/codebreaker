package input

import (
	"io"
	"os"

	"golang.org/x/term"
)

// hiddenInput reads one line from a real terminal without showing what's
// typed, and can put the terminal back to how it was found.
//
// It's an interface, rather than calling package term directly, so tests
// can swap in a fake: a test can't open a real terminal.
type hiddenInput interface {
	// ReadHidden reads one line with echo turned off. It runs on the
	// reader goroutine.
	ReadHidden() (string, error)
	// Restore puts the terminal back to its state when the Terminal was
	// created, with echo on. It's safe to call at any time.
	Restore()
}

// tty is the real hiddenInput, for a terminal on file descriptor fd.
type tty struct {
	fd    int
	state *term.State // the terminal's settings at start-up, with echo on
}

// newTTY returns a hiddenInput if in is a terminal, or nil if it isn't
// (a pipe, a file, or a string in tests). Then Secret reads an ordinary
// line instead.
func newTTY(in io.Reader) hiddenInput {
	// Go note: `f, ok := in.(*os.File)` is a type assertion. It asks
	// whether the interface value in holds an *os.File underneath. If it
	// does, ok is true and f is that file; if not, ok is false. Only a
	// file has a descriptor (Fd) that the terminal functions can use.
	f, ok := in.(*os.File)
	if !ok {
		return nil
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	state, err := term.GetState(fd)
	if err != nil {
		return nil
	}
	// Go note: newTTY returns the interface type hiddenInput. Returning the
	// literal nil above gives a true nil interface, which callers can
	// compare with nil. Returning a nil *tty instead would give an
	// interface that holds a nil pointer and is NOT equal to nil, a
	// classic Go surprise.
	return &tty{fd: fd, state: state}
}

// ReadHidden reads a line without echo, using term.ReadPassword. Ctrl-C
// still sends an interrupt while it waits.
func (t *tty) ReadHidden() (string, error) {
	b, err := term.ReadPassword(t.fd)
	return string(b), err
}

// Restore turns the terminal's saved settings back on. The error is
// ignored: if the terminal can't be restored, there's nothing more to try.
func (t *tty) Restore() {
	_ = term.Restore(t.fd, t.state)
}

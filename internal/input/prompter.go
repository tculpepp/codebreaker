package input

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ErrQuit is returned by Line when the player types q or quit. Callers pass
// it up to the app, which shows the standings and exits normally (D16).
var ErrQuit = errors.New("input: quit")

// ErrHelp is returned by Line when the player types ? or help. Callers show
// the help text and ask the same question again.
var ErrHelp = errors.New("input: help requested")

// Prompter asks the player a question and returns their one-line answer.
//
// Line writes prompt, waits for a line, and returns it without leading or
// trailing spaces. It returns ErrQuit or ErrHelp for those commands,
// io.EOF when the input has ended, and ctx.Err() if ctx ends first.
//
// Secret is the same, but in a real terminal nothing typed is shown on
// screen. It's for a human Code Maker typing a code that the other player
// mustn't see (D10).
//
// Go note: package game declares its interfaces where they're used, and
// that's the usual Go style. Prompter is declared here instead, next to its
// implementation, because two packages use it (player and app) and both
// need exactly the same methods. Declaring it once avoids two identical
// copies. Either way, nothing has to say "implements Prompter": having
// matching methods is enough.
type Prompter interface {
	Line(ctx context.Context, prompt string) (string, error)
	Secret(ctx context.Context, prompt string) (string, error)
}

// result is one item delivered by the background reader: a line of text,
// or the error that stopped reading.
type result struct {
	text string
	err  error
}

// Terminal is a Prompter that reads lines from an io.Reader (normally the
// keyboard, os.Stdin) and writes prompts to an io.Writer (normally the
// screen, os.Stdout).
//
// A Terminal is meant to be used by one goroutine at a time: the game asks
// one question, gets one answer, then asks the next.
type Terminal struct {
	in   io.Reader
	out  io.Writer
	echo bool        // write each line after the prompt (scripted input only)
	tty  hiddenInput // hidden input, or nil when in isn't a terminal
	eof  bool        // the input has ended; every later question gets io.EOF

	start    sync.Once   // starts the background reader on the first question
	requests chan bool   // asks the reader for one line; true means hidden
	lines    chan result // the reader's answers, closed at end of input
}

// Go note: this is a compile-time check that *Terminal satisfies Prompter.
// If a method's signature drifted, the build would fail here with a clear
// message instead of somewhere far away.
var _ Prompter = (*Terminal)(nil)

// NewTerminal returns a Prompter reading from in and writing prompts to
// out. It doesn't echo input, because a real terminal already shows what
// the user types. When in is a terminal, Secret hides what's typed.
//
// Go note: in is an io.Reader, an interface with a single method,
// Read([]byte) (int, error). os.Stdin, a strings.Reader and a network
// connection all satisfy it, so the same code reads from the keyboard in
// the game and from a string in tests.
func NewTerminal(in io.Reader, out io.Writer) *Terminal {
	return &Terminal{
		in:  in,
		out: out,
		tty: newTTY(in),
		// Go note: a buffer of 1 lets a question be sent even when the
		// reader has already stopped (after a read error), so the
		// question then sees the closed lines channel instead of waiting
		// forever for a reader that's gone.
		requests: make(chan bool, 1),
		lines:    make(chan result),
	}
}

// NewScripted returns a Prompter that answers with lines, in order, and
// then reports io.EOF. It echoes each line after its prompt, so out reads
// like a transcript of a real session; answers to Secret appear as
// "(hidden)". It's for tests and demos.
func NewScripted(out io.Writer, lines ...string) *Terminal {
	// Go note: strings.Builder builds a string piece by piece without
	// making a new string for every +. Its zero value is ready to use.
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	t := NewTerminal(strings.NewReader(b.String()), out)
	t.echo = true
	return t
}

// Line writes prompt and returns the next line typed, trimmed of spaces.
// See Prompter for the errors it returns.
//
// Line returns as soon as ctx ends, even while it's waiting for the player
// to type. That is what makes Ctrl-C work at any prompt.
func (t *Terminal) Line(ctx context.Context, prompt string) (string, error) {
	return t.ask(ctx, prompt, false)
}

// Secret is like Line, but in a real terminal what's typed isn't shown.
// The terminal is always put back afterwards, even after Ctrl-C, so the
// player's shell never stays silent.
func (t *Terminal) Secret(ctx context.Context, prompt string) (string, error) {
	if t.tty != nil {
		// Go note: defer runs this when Secret returns, on every path:
		// a normal answer, an error, or Ctrl-C. That's what makes it the
		// right tool for cleanup that must never be skipped.
		defer t.tty.Restore()
	}
	return t.ask(ctx, prompt, true)
}

// ask does the work of Line and Secret: show the prompt, ask the reader
// goroutine for one line (hidden or not), and wait for the answer or for
// ctx to end.
func (t *Terminal) ask(ctx context.Context, prompt string, hidden bool) (string, error) {
	// Don't ask a question that can't be answered.
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t.eof {
		return "", io.EOF
	}

	// Go note: sync.Once runs the function the first time Do is called and
	// never again, however many times ask is called. The reader is started
	// lazily, here, rather than in NewTerminal, so building a Terminal
	// that's never used starts nothing.
	//
	// Go note: `go f()` starts f in a new goroutine, a lightweight thread
	// managed by the Go runtime. It runs at the same time as this code.
	t.start.Do(func() { go t.readLines() })

	// Writes to the screen can only fail if the terminal has gone away, and
	// there would be nobody left to tell, so the error is ignored.
	fmt.Fprint(t.out, prompt)

	// Go note: select waits on several channel operations and runs the
	// first one that's ready. Here it first sends the request (or gives up
	// on Ctrl-C), then waits for either the answer or the end of ctx. A
	// plain blocking read of the keyboard couldn't be interrupted this
	// way; moving the read into a goroutine is what makes it possible.
	select {
	case t.requests <- hidden:
	case <-ctx.Done():
		fmt.Fprintln(t.out)
		return "", ctx.Err()
	}

	select {
	case <-ctx.Done():
		fmt.Fprintln(t.out) // end the prompt line so later output starts cleanly
		return "", ctx.Err()

	// Go note: `r, ok := <-ch` is the "comma ok" receive. ok is false once
	// the channel has been closed and emptied, which is how the reader
	// says "no more input".
	case r, ok := <-t.lines:
		switch {
		case !ok:
			t.eof = true
			fmt.Fprintln(t.out)
			return "", io.EOF
		case r.err != nil:
			fmt.Fprintln(t.out)
			return "", r.err
		}

		switch {
		case hidden && t.tty != nil:
			fmt.Fprintln(t.out) // the Enter key wasn't echoed either
		case hidden && t.echo:
			fmt.Fprintln(t.out, "(hidden)") // never show a secret, even in a transcript
		case t.echo:
			fmt.Fprintln(t.out, r.text)
		}
		return command(r.text)
	}
}

// readLines runs in its own goroutine. Each time ask sends a request, it
// reads exactly one line, hidden or not, and sends it on t.lines. At the
// end of the input it closes t.lines and stops.
//
// It never reads ahead. If it did, it would already be waiting on the
// keyboard when a hidden read started, and the two would compete for the
// player's keystrokes. Mixing the two kinds of read is safe because a
// terminal hands over at most one line per read, so the Scanner never
// holds typed-ahead text that a hidden read would miss.
//
// If the player presses Ctrl-C, ask stops waiting but this goroutine stays
// blocked reading the keyboard. That's fine: the program is about to exit,
// and a goroutine costs only a few kilobytes.
func (t *Terminal) readLines() {
	// Go note: defer runs close(t.lines) when readLines returns, however it
	// returns. Closing a channel tells every receiver "nothing more is
	// coming"; only the sender should ever close it.
	defer close(t.lines)

	// Go note: bufio.Scanner splits a stream into lines. Scan reads the
	// next one and returns false at the end of input or on an error;
	// Text returns the line without its "\n" (or "\r\n" on Windows).
	sc := bufio.NewScanner(t.in)

	// Go note: ranging over a channel receives values one at a time until
	// the channel is closed. requests is never closed, so this loop only
	// ends through a return.
	for hidden := range t.requests {
		if hidden && t.tty != nil {
			text, err := t.tty.ReadHidden()
			if err != nil {
				err = fmt.Errorf("input: reading hidden input: %w", err)
			}
			t.lines <- result{text: text, err: err}
			continue
		}
		if !sc.Scan() {
			// Err is nil at a normal end of input, which the deferred
			// close reports as io.EOF. Any other error is passed on first.
			if err := sc.Err(); err != nil {
				t.lines <- result{err: fmt.Errorf("input: reading: %w", err)}
			}
			return
		}
		t.lines <- result{text: sc.Text()}
	}
}

// command trims text and turns the quit and help commands, in any case,
// into ErrQuit and ErrHelp. Anything else comes back as trimmed text.
func command(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch strings.ToLower(text) {
	case "q", "quit":
		return "", ErrQuit
	case "?", "help":
		return "", ErrHelp
	}
	return text, nil
}

package input

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTTY stands in for a real terminal's hidden input, which tests can't
// open. It returns a fixed answer (or error), can be made to block until
// released, and counts how often the terminal was restored.
type fakeTTY struct {
	text  string
	err   error
	block chan struct{} // if not nil, ReadHidden waits until it's closed

	// Go note: ReadHidden runs on the reader goroutine and Restore on the
	// test's goroutine. atomic.Int32 can be changed from both safely; a
	// plain int would be a data race, which `go test -race` reports.
	restored atomic.Int32
}

// Go note: compile-time check that the fake satisfies hiddenInput.
var _ hiddenInput = (*fakeTTY)(nil)

// ReadHidden returns the scripted answer, after block is closed if set.
func (f *fakeTTY) ReadHidden() (string, error) {
	if f.block != nil {
		<-f.block
	}
	return f.text, f.err
}

// Restore counts the call.
func (f *fakeTTY) Restore() { f.restored.Add(1) }

// TestSecretScripted checks Secret without a terminal: it reads an
// ordinary line, and the transcript shows "(hidden)" instead of the code,
// so even test output never reveals a secret.
func TestSecretScripted(t *testing.T) {
	var out strings.Builder
	p := NewScripted(&out, " rgybk ")
	got, err := p.Secret(context.Background(), "Code ▸ ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "rgybk" {
		t.Errorf("Secret = %q, want %q", got, "rgybk")
	}
	if want := "Code ▸ (hidden)\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestSecretCommands checks that quit and help work at a secret prompt
// too, like at every other prompt.
func TestSecretCommands(t *testing.T) {
	for typed, want := range map[string]error{"q": ErrQuit, "?": ErrHelp} {
		if _, err := NewScripted(io.Discard, typed).Secret(context.Background(), "> "); !errors.Is(err, want) {
			t.Errorf("Secret(%q) error = %v, want %v", typed, err, want)
		}
	}
}

// TestSecretUsesTerminal checks the terminal path with a fake: the hidden
// reader supplies the answer, a newline ends the prompt (the Enter key
// wasn't echoed), the terminal is restored, and ordinary lines still work
// afterwards.
func TestSecretUsesTerminal(t *testing.T) {
	var out strings.Builder
	p := NewTerminal(strings.NewReader("after\n"), &out)
	tty := &fakeTTY{text: "wpogr"}
	p.tty = tty

	got, err := p.Secret(context.Background(), "Code ▸ ")
	if err != nil || got != "wpogr" {
		t.Fatalf("Secret = %q, %v; want %q, nil", got, err, "wpogr")
	}
	if want := "Code ▸ \n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if tty.restored.Load() != 1 {
		t.Errorf("terminal restored %d times, want 1", tty.restored.Load())
	}
	if got, err := p.Line(context.Background(), "> "); err != nil || got != "after" {
		t.Errorf("Line after Secret = %q, %v; want %q, nil", got, err, "after")
	}
}

// TestSecretTerminalError checks that a failed hidden read is reported,
// and the terminal is restored anyway.
func TestSecretTerminalError(t *testing.T) {
	errBoom := errors.New("boom")
	p := NewTerminal(strings.NewReader(""), io.Discard)
	tty := &fakeTTY{err: errBoom}
	p.tty = tty

	if _, err := p.Secret(context.Background(), "> "); !errors.Is(err, errBoom) {
		t.Errorf("Secret error = %v, want one wrapping %v", err, errBoom)
	}
	if tty.restored.Load() != 1 {
		t.Errorf("terminal restored %d times, want 1", tty.restored.Load())
	}
}

// TestSecretCancelRestoresTerminal is the important safety case: Ctrl-C
// while typing a hidden code must still turn echo back on, or the
// player's shell would be left not showing what they type.
func TestSecretCancelRestoresTerminal(t *testing.T) {
	tty := &fakeTTY{block: make(chan struct{})}
	t.Cleanup(func() { close(tty.block) }) // lets the reader goroutine finish
	p := NewTerminal(strings.NewReader(""), io.Discard)
	p.tty = tty

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.Secret(ctx, "> "); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Secret error = %v, want context.DeadlineExceeded", err)
	}
	if tty.restored.Load() != 1 {
		t.Errorf("terminal restored %d times, want 1", tty.restored.Load())
	}
}

// TestNewTTYNeedsATerminal checks that hidden input is only used for a
// real terminal. A string or an ordinary file gets nil, so Secret falls
// back to reading a plain line.
func TestNewTTYNeedsATerminal(t *testing.T) {
	if tty := newTTY(strings.NewReader("")); tty != nil {
		t.Errorf("newTTY(strings.Reader) = %v, want nil", tty)
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if tty := newTTY(f); tty != nil {
		t.Errorf("newTTY(regular file) = %v, want nil", tty)
	}
}

// lineReader returns one line per Read call, like a terminal does, and
// counts the calls.
type lineReader struct {
	lines []string
	reads atomic.Int32
}

// Read hands out the next line, or io.EOF when there are none left.
func (r *lineReader) Read(p []byte) (int, error) {
	n := int(r.reads.Add(1))
	if n > len(r.lines) {
		return 0, io.EOF
	}
	return copy(p, r.lines[n-1]+"\n"), nil
}

// TestReaderDoesNotReadAhead checks that the background reader only reads
// when asked. If it read ahead, it would be waiting on the keyboard when a
// hidden read started, and the two would fight over the next keystrokes.
func TestReaderDoesNotReadAhead(t *testing.T) {
	in := &lineReader{lines: []string{"one", "two"}}
	p := NewTerminal(in, io.Discard)
	if _, err := p.Line(context.Background(), "> "); err != nil {
		t.Fatal(err)
	}
	// Give a reading-ahead goroutine time to make its next call.
	time.Sleep(20 * time.Millisecond)
	if got := in.reads.Load(); got != 1 {
		t.Errorf("input read %d times after one Line, want 1", got)
	}
}

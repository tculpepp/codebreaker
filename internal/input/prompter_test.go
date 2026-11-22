package input

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// TestLineReturnsTrimmedText checks that Line hands back what was typed,
// without surrounding spaces, and leaves inner spaces alone (Parse accepts
// "r g y b k", so they must survive).
func TestLineReturnsTrimmedText(t *testing.T) {
	p := NewScripted(io.Discard, "  r g y b k \t")
	got, err := p.Line(context.Background(), "> ")
	if err != nil {
		t.Fatal(err)
	}
	if want := "r g y b k"; got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

// TestLineReadsInOrder checks that successive calls return successive
// lines, so a script plays out exactly as written.
func TestLineReadsInOrder(t *testing.T) {
	p := NewScripted(io.Discard, "one", "two", "three")
	for _, want := range []string{"one", "two", "three"} {
		got, err := p.Line(context.Background(), "> ")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Line = %q, want %q", got, want)
		}
	}
}

// TestLineCommands checks that the quit and help commands work in any
// case and with stray spaces, because they must work at every prompt
// (requirements.md "Error Handling").
func TestLineCommands(t *testing.T) {
	tests := []struct {
		name  string
		typed string
		want  error
	}{
		{name: "q", typed: "q", want: ErrQuit},
		{name: "Q upper case", typed: "Q", want: ErrQuit},
		{name: "quit", typed: "quit", want: ErrQuit},
		{name: "QUIT with spaces", typed: "  QUIT ", want: ErrQuit},
		{name: "question mark", typed: "?", want: ErrHelp},
		{name: "help", typed: "help", want: ErrHelp},
		{name: "Help mixed case", typed: "Help", want: ErrHelp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewScripted(io.Discard, tt.typed)
			got, err := p.Line(context.Background(), "> ")
			if !errors.Is(err, tt.want) {
				t.Errorf("Line error = %v, want %v", err, tt.want)
			}
			if got != "" {
				t.Errorf("Line text = %q, want empty with a command", got)
			}
		})
	}
}

// TestLineNotACommand checks that words merely starting like a command are
// ordinary text. Only the exact commands are special.
func TestLineNotACommand(t *testing.T) {
	for _, typed := range []string{"qu", "quitter", "??", "helpme"} {
		p := NewScripted(io.Discard, typed)
		got, err := p.Line(context.Background(), "> ")
		if err != nil || got != typed {
			t.Errorf("Line(%q) = %q, %v; want the text back and no error", typed, got, err)
		}
	}
}

// TestLineEOF checks that running out of input gives io.EOF, and keeps
// giving it. The app treats EOF like quitting, so a piped script that ends
// early finishes cleanly.
func TestLineEOF(t *testing.T) {
	p := NewScripted(io.Discard)
	for i := range 2 {
		if _, err := p.Line(context.Background(), "> "); err != io.EOF {
			t.Errorf("call %d: Line error = %v, want io.EOF", i+1, err)
		}
	}
}

// TestScriptedEchoes checks that the scripted prompter writes the prompt
// and then the "typed" line, so a test transcript reads like a real
// session.
func TestScriptedEchoes(t *testing.T) {
	var out strings.Builder
	p := NewScripted(&out, "hello")
	if _, err := p.Line(context.Background(), "Name ▸ "); err != nil {
		t.Fatal(err)
	}
	if want := "Name ▸ hello\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestScriptedEOFEndsLine checks that hitting the end of a script still
// ends the prompt line, so the next output doesn't run onto it.
func TestScriptedEOFEndsLine(t *testing.T) {
	var out strings.Builder
	p := NewScripted(&out)
	_, _ = p.Line(context.Background(), "Name ▸ ")
	if want := "Name ▸ \n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestTerminalDoesNotEcho checks that a real terminal prompter prints only
// the prompt. The terminal itself shows what the user types, so echoing it
// again would print it twice.
func TestTerminalDoesNotEcho(t *testing.T) {
	var out strings.Builder
	p := NewTerminal(strings.NewReader("hello\n"), &out)
	if _, err := p.Line(context.Background(), "Name ▸ "); err != nil {
		t.Fatal(err)
	}
	if want := "Name ▸ "; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestLineCancelWhileWaiting is the Ctrl-C case: nothing is ever typed,
// and Line must still return as soon as the context ends, instead of
// waiting forever for a line.
func TestLineCancelWhileWaiting(t *testing.T) {
	// Go note: io.Pipe gives a reader that blocks until someone writes to
	// the other end. Nobody writes here, so it behaves like a keyboard
	// nobody touches.
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() }) // lets the background reader finish

	var out strings.Builder
	p := NewTerminal(pr, &out)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := p.Line(ctx, "> ")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Line error = %v, want context.DeadlineExceeded", err)
	}
	if want := "> \n"; out.String() != want {
		t.Errorf("output = %q, want %q (prompt line ended)", out.String(), want)
	}
}

// TestLineAlreadyCancelled checks that a cancelled context wins even when
// input is ready, and that no prompt is printed for a question that will
// never be answered.
func TestLineAlreadyCancelled(t *testing.T) {
	var out strings.Builder
	p := NewScripted(&out, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Line(ctx, "> ")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Line error = %v, want context.Canceled", err)
	}
	if out.String() != "" {
		t.Errorf("output = %q, want nothing", out.String())
	}
}

// TestLineReadError checks that a failing input stream is reported, with
// the cause still visible to errors.Is, and that later calls see EOF.
func TestLineReadError(t *testing.T) {
	errBoom := errors.New("boom")
	p := NewTerminal(iotest.ErrReader(errBoom), io.Discard)

	_, err := p.Line(context.Background(), "> ")
	if !errors.Is(err, errBoom) {
		t.Fatalf("Line error = %v, want one wrapping %v", err, errBoom)
	}
	if _, err := p.Line(context.Background(), "> "); err != io.EOF {
		t.Errorf("second Line error = %v, want io.EOF", err)
	}
}

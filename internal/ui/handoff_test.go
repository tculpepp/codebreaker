package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// TestClearPlain checks that, when output isn't a terminal (piped, or a
// test), Clear draws a separator line instead of sending escape codes
// that would show up as junk in a file.
func TestClearPlain(t *testing.T) {
	r, out := newTestRenderer()
	r.Clear()
	if strings.Contains(out.String(), "\x1b") {
		t.Errorf("plain Clear wrote escape codes: %q", out.String())
	}
	if !strings.Contains(out.String(), "────") {
		t.Errorf("plain Clear = %q, want a separator line", out.String())
	}
}

// TestClearTerminal checks that, in a real terminal, Clear wipes the
// screen and the scrollback, so the next player can't scroll up to see
// anything left from the previous one (D10).
func TestClearTerminal(t *testing.T) {
	r, out := newTestRenderer()
	r.tty = true
	r.Clear()
	if want := "\x1b[H\x1b[2J\x1b[3J"; out.String() != want {
		t.Errorf("terminal Clear = %q, want %q", out.String(), want)
	}
}

// TestHandoff checks the screen shown when the keyboard passes from one
// human to the other: it clears, then names who should take over.
func TestHandoff(t *testing.T) {
	r, out := newTestRenderer()
	r.Handoff("Ann")
	checkGolden(t, "handoff", out.String())
}

// TestRedraw checks that Redraw draws the current round's board again,
// with every row so far, as Board would.
func TestRedraw(t *testing.T) {
	r, out := newTestRenderer()
	turn := game.Turn{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Partial: 3}}
	r.RoundStarted(singleInfo)
	r.TurnPlayed(singleInfo, turn)
	out.Reset()
	r.Redraw()

	want, wantOut := newTestRenderer()
	want.Board(singleInfo, []game.Turn{turn})
	if out.String() != wantOut.String() {
		t.Errorf("Redraw =\n%s\nwant\n%s", out.String(), wantOut.String())
	}
}

// TestIsTerminal checks terminal detection for writers that aren't
// terminals: a string builder and an ordinary file.
func TestIsTerminal(t *testing.T) {
	if isTerminal(&strings.Builder{}) {
		t.Error("isTerminal(strings.Builder) = true, want false")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("isTerminal(regular file) = true, want false")
	}
}

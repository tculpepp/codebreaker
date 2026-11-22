package ui

import (
	"errors"
	"strings"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// allowColor sets the environment so that only Options decide about
// color, whatever the machine running the tests has set.
//
// Go note: t.Setenv changes an environment variable for this test only and
// puts it back afterwards. A test that uses it can't run in parallel with
// others, which Go enforces.
func allowColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
}

// drawEverything sends r through every kind of screen: a round with
// turns, a summary, the end of a match, help, setup, a menu, an error and
// a handoff-free redraw.
func drawEverything(t *testing.T, r *Renderer) {
	t.Helper()
	info := game.RoundInfo{Number: 2, Mode: game.ModeMatch, MaxRows: game.MaxRows,
		Maker: computer, Breaker: tom}
	r.Welcome()
	r.Setup(game.ModeMatch, [2]game.Seat{tom, computer}, 1, true)
	r.RoundStarted(info)
	r.TurnPlayed(info, game.Turn{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Exact: 1, Partial: 2}})
	r.TurnPlayed(info, game.Turn{Row: 2, Guess: mustParse(t, "WPOGR"), Feedback: code.Feedback{Exact: 5}})
	r.Error(errors.New("something to fix"))
	r.RoundEnded(game.RoundResult{Number: 2, MakerIdx: 1, BreakerIdx: 0, Rows: 2, Solved: true,
		Secret: mustParse(t, "WPOGR"), Points: 12}, [2]game.Seat{tom, computer})
	r.MatchEnded(game.Standings{Seats: [2]game.Seat{tom, computer}, Results: make([]game.RoundResult, 2), Winner: -1})
	r.Help()
	r.Menu("Play again?", []string{"New game", "Quit"})
	r.write(r.Prompt("Choose " + r.Range(1, 2)))
}

// TestColorMatchesPlain is the key color test: the colored output, with
// its escape codes removed, must be exactly the plain output. So color
// only ever adds to what's on screen, the board still lines up, and the
// game reads the same without it.
func TestColorMatchesPlain(t *testing.T) {
	allowColor(t)
	var colored, plain strings.Builder
	drawEverything(t, NewRenderer(&colored, Options{ForceTTY: true}))
	drawEverything(t, NewRenderer(&plain, Options{}))

	if !strings.Contains(colored.String(), "\x1b[38;5;196m") {
		t.Fatalf("colored output has no red in it:\n%q", colored.String())
	}
	if got := stripANSI(colored.String()); got != plain.String() {
		t.Errorf("colored output without codes differs from plain\n--- stripped ---\n%s--- plain ---\n%s", got, plain.String())
	}
}

// TestPegColors checks a few pegs against the palette in
// implementation-plan.md §3. Black is drawn gray (240) so it shows on dark
// terminals.
func TestPegColors(t *testing.T) {
	allowColor(t)
	r := NewRenderer(&strings.Builder{}, Options{ForceTTY: true})
	tests := []struct {
		c    code.Color
		want string
	}{
		{code.Black, "\x1b[38;5;240m●K\x1b[0m"},
		{code.Red, "\x1b[38;5;196m●R\x1b[0m"},
		{code.Orange, "\x1b[38;5;208m●O\x1b[0m"},
	}
	for _, tt := range tests {
		if got := r.Peg(tt.c); got != tt.want {
			t.Errorf("Peg(%v) = %q, want %q", tt.c, got, tt.want)
		}
	}
	if got, want := r.Feedback(code.Feedback{Exact: 1, Partial: 1}),
		"\x1b[1m●\x1b[0m \x1b[38;5;226m△\x1b[0m \x1b[2m·\x1b[0m \x1b[2m·\x1b[0m \x1b[2m·\x1b[0m"; got != want {
		t.Errorf("Feedback = %q, want %q", got, want)
	}
}

// TestColorTurnedOff checks every way color is turned off: the NoColor
// option, the NO_COLOR environment variable (https://no-color.org), a
// "dumb" terminal, and output that isn't a terminal at all.
func TestColorTurnedOff(t *testing.T) {
	tests := []struct {
		name    string
		noColor string
		term    string
		opts    Options
	}{
		{name: "option", term: "xterm", opts: Options{ForceTTY: true, NoColor: true}},
		{name: "NO_COLOR", noColor: "1", term: "xterm", opts: Options{ForceTTY: true}},
		{name: "dumb terminal", term: "dumb", opts: Options{ForceTTY: true}},
		{name: "not a terminal", term: "xterm", opts: Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("TERM", tt.term)
			r := NewRenderer(&strings.Builder{}, tt.opts)
			if got := r.Peg(code.Red); got != "●R" {
				t.Errorf("Peg(Red) = %q, want plain %q", got, "●R")
			}
		})
	}
}

// TestRedrawInPlace checks that in a terminal each turn clears the screen
// before drawing the board, so the board stays in one place instead of
// scrolling. A new round doesn't clear, so the last round's summary stays
// readable until the player answers the next prompt.
func TestRedrawInPlace(t *testing.T) {
	var out strings.Builder
	r := NewRenderer(&out, Options{ForceTTY: true, NoColor: true})
	r.RoundStarted(singleInfo)
	if strings.Contains(out.String(), ansiClearScreen) {
		t.Errorf("RoundStarted cleared the screen")
	}
	out.Reset()
	r.TurnPlayed(singleInfo, game.Turn{Row: 1, Guess: mustParse(t, "RGYBK")})
	if !strings.HasPrefix(out.String(), ansiHome+ansiClearScreen) {
		t.Errorf("TurnPlayed didn't start by clearing: %q", out.String()[:min(20, len(out.String()))])
	}
}

// TestASCII checks the ASCII-only mode for terminals without Unicode: the
// same screens with * ^ . for feedback, + - | for the board, and no peg
// symbol, and not a single non-ASCII byte anywhere.
func TestASCII(t *testing.T) {
	var out strings.Builder
	r := NewRenderer(&out, Options{ASCII: true})
	drawEverything(t, r)
	for i, b := range []byte(out.String()) {
		if b >= 0x80 {
			t.Fatalf("non-ASCII byte %#x at %d:\n%s", b, i, out.String())
		}
	}
	out.Reset()
	r.Board(game.RoundInfo{Number: 3, Mode: game.ModeMatch, MaxRows: game.MaxRows, Maker: computer, Breaker: tom},
		[]game.Turn{{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Exact: 1, Partial: 2}}})
	checkGolden(t, "board_ascii", out.String())
}

// TestPromptAndRange checks the two small helpers other packages use so
// their prompts follow the symbol set.
func TestPromptAndRange(t *testing.T) {
	u := NewRenderer(&strings.Builder{}, Options{})
	a := NewRenderer(&strings.Builder{}, Options{ASCII: true})
	if got, want := u.Prompt("Row 1/12"), " Row 1/12 ▸ "; got != want {
		t.Errorf("Prompt = %q, want %q", got, want)
	}
	if got, want := a.Prompt("Row 1/12"), " Row 1/12 > "; got != want {
		t.Errorf("ASCII Prompt = %q, want %q", got, want)
	}
	if got, want := u.Range(1, 3), "1–3"; got != want {
		t.Errorf("Range = %q, want %q", got, want)
	}
	if got, want := a.Range(1, 3), "1-3"; got != want {
		t.Errorf("ASCII Range = %q, want %q", got, want)
	}
}

// TestEnvAllowsColor checks the environment rules on their own.
func TestEnvAllowsColor(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM": "xterm"}, true},
		{map[string]string{}, true}, // Windows usually has no TERM
		{map[string]string{"TERM": "dumb"}, false},
		{map[string]string{"NO_COLOR": "1", "TERM": "xterm"}, false},
		{map[string]string{"NO_COLOR": "", "TERM": "xterm"}, true}, // empty means unset
	}
	for _, tt := range tests {
		getenv := func(k string) string { return tt.env[k] }
		if got := envAllowsColor(getenv); got != tt.want {
			t.Errorf("envAllowsColor(%v) = %v, want %v", tt.env, got, tt.want)
		}
	}
}

// TestStripANSI checks the helper the color test relies on: it removes
// color and cursor codes and leaves everything else alone.
func TestStripANSI(t *testing.T) {
	in := "\x1b[H\x1b[2J\x1b[38;5;196m●R\x1b[0m ok"
	if got, want := stripANSI(in), "●R ok"; got != want {
		t.Errorf("stripANSI = %q, want %q", got, want)
	}
}

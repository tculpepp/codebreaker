package ui

import (
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// Options choose how a Renderer draws. The zero value, Options{}, means
// "decide for me": Unicode symbols, with color and in-place redrawing only
// when the output is a terminal that supports them.
type Options struct {
	// NoColor turns color off even in a terminal (the -no-color flag).
	NoColor bool
	// ASCII uses only ASCII characters, for terminals or fonts without the
	// Unicode symbols (the -ascii flag).
	ASCII bool
	// ForceTTY treats the output as a terminal even when it isn't, so tests
	// and demos can see the colored, redrawn output.
	ForceTTY bool
}

// symbols is one complete set of the characters the screens are drawn
// with. Every screen takes its symbols from the Renderer's set, so the
// ASCII mode can swap all of them at once.
type symbols struct {
	exact, partial, empty string // feedback: right spot, wrong spot, no match
	peg                   string // drawn before each code peg's letter
	dot                   string // separates items on a line
	dash                  string // between the ends of a range, as in 5–6
	arrow                 string // ends every prompt
	hr                    string // horizontal line, for the board and separators
	v                     string // vertical line of the board
	// The board's corners and junctions, named by row (top, middle,
	// bottom) and column (left, middle, right).
	tl, tm, tr, ml, mm, mr, bl, bm, br string
}

// unicodeSymbols is the normal set (requirements.md "Visual Elements").
var unicodeSymbols = symbols{
	exact: "●", partial: "△", empty: "·", peg: "●",
	dot: "·", dash: "–", arrow: "▸", hr: "─", v: "│",
	tl: "┌", tm: "┬", tr: "┐", ml: "├", mm: "┼", mr: "┤", bl: "└", bm: "┴", br: "┘",
}

// asciiSymbols is the -ascii set. Pegs show just their letter.
var asciiSymbols = symbols{
	exact: "*", partial: "^", empty: ".", peg: "",
	dot: "|", dash: "-", arrow: ">", hr: "-", v: "|",
	tl: "+", tm: "+", tr: "+", ml: "+", mm: "+", mr: "+", bl: "+", bm: "+", br: "+",
}

// SGR ("Select Graphic Rendition") codes: the part of an ANSI escape code
// that sets text style. "38;5;N" picks color N from the 256-color palette.
const (
	sgrBold   = "1"
	sgrDim    = "2"
	sgrYellow = "38;5;226"
	sgrRed    = "38;5;196"
)

// pegColors is the 256-color palette entry for each peg color
// (implementation-plan.md §3). Black is drawn gray so it shows on a dark
// background.
var pegColors = [code.NumColors]string{
	code.Black:  "38;5;240",
	code.White:  "38;5;255",
	code.Red:    "38;5;196",
	code.Blue:   "38;5;33",
	code.Green:  "38;5;40",
	code.Yellow: "38;5;226",
	code.Pink:   "38;5;205",
	code.Orange: "38;5;208",
}

// scoringPegColor returns the palette entry for a player's scoring peg.
func scoringPegColor(p game.PegColor) string {
	if p == game.PegBlue {
		return pegColors[code.Blue]
	}
	return pegColors[code.Red]
}

// paint wraps s in the escape codes for style sgr, and a reset afterwards,
// when color is on. Otherwise it returns s unchanged.
func (r *Renderer) paint(sgr, s string) string {
	if !r.color || s == "" {
		return s
	}
	return "\x1b[" + sgr + "m" + s + "\x1b[0m"
}

// envAllowsColor reports whether the environment permits color: NO_COLOR
// must be unset or empty (https://no-color.org), and TERM must not be
// "dumb". getenv is os.Getenv in the game and a fake in tests.
//
// Go note: taking the lookup as a function value, instead of calling
// os.Getenv directly, lets a test supply any environment it likes.
func envAllowsColor(getenv func(string) string) bool {
	return getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"
}

// newStyle works out the Renderer's settings from o and w: whether w is a
// terminal (redraw in place, clear the screen), whether to use color, and
// which symbols to draw with.
func newStyle(w io.Writer, o Options) (tty, color bool, sym symbols) {
	// enableVT turns on escape-code support where it's off by default
	// (older Windows consoles). If it can't, the output is treated as
	// plain text.
	tty = o.ForceTTY || (isTerminal(w) && enableVT(w))
	color = tty && !o.NoColor && envAllowsColor(os.Getenv)
	sym = unicodeSymbols
	if o.ASCII {
		sym = asciiSymbols
	}
	return tty, color, sym
}

// ansiCode matches one ANSI escape code: ESC, "[", numbers and semicolons,
// then a letter (m for style, H and J for the cursor and clearing).
//
// Go note: regexp.MustCompile compiles a regular expression once, when the
// program starts, and panics if it's invalid. Using it in a package-level
// variable is the standard pattern: a typo is caught the first time any
// test runs, and the compiled pattern is reused on every call.
var ansiCode = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// stripANSI removes every escape code from s, leaving the visible text.
func stripANSI(s string) string {
	return ansiCode.ReplaceAllString(s, "")
}

// padRight adds spaces after s until it's width characters wide on screen.
// Escape codes take no space on screen, so they aren't counted.
//
// Go note: fmt's %-*s padding would count the bytes of the escape codes
// as characters and push colored cells out of line. Counting the runes of
// the stripped text gives the true width.
func padRight(s string, width int) string {
	gap := width - utf8.RuneCountInString(stripANSI(s))
	return s + strings.Repeat(" ", max(gap, 0))
}

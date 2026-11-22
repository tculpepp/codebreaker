package ui

import (
	"fmt"
	"io"
	"strings"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// The feedback symbols (requirements.md D9). Exact matches come first,
// then partial ones, then empty slots, so the order never gives away which
// peg matched.
const (
	symExact   = "●" // right color, right spot (black key peg)
	symPartial = "△" // right color, wrong spot (white key peg)
	symEmpty   = "·" // no match for this slot
	symPeg     = "●" // a code peg; colored in v1.0.0
)

// Board column widths, in characters, not counting the │ borders. The
// guess column fits 5 pegs of 2 characters with spaces between and a space
// either side: 1 + 14 + 1 = 16.
const (
	colRow    = 5
	colGuess  = 16
	colResult = 8
)

// Renderer draws the game as plain text on an io.Writer.
//
// It remembers the current round's mode and the turns played so far,
// because the Observer events only carry the newest turn and the board
// shows them all.
type Renderer struct {
	w     io.Writer
	mode  game.Mode   // mode of the latest round, used by the summaries
	turns []game.Turn // turns played in the current round, oldest first
}

// Go note: a compile-time check that *Renderer satisfies game.Observer, so
// it can be passed straight to game.NewMatch.
var _ game.Observer = (*Renderer)(nil)

// NewRenderer returns a Renderer that writes to w.
func NewRenderer(w io.Writer) *Renderer {
	return &Renderer{w: w}
}

// write sends s to the output in one call.
//
// Errors are ignored: a terminal write only fails if the terminal has gone
// away, and then there's no one to show an error to. Observer methods have
// no error result for the same reason.
func (r *Renderer) write(s string) {
	// Go note: io.WriteString works with any io.Writer. If the writer has
	// its own WriteString method (strings.Builder and os.File do), it uses
	// that and skips converting s to a []byte.
	io.WriteString(r.w, s)
}

// Peg returns one code peg as it appears on screen: a peg symbol followed
// by the color's letter, such as "●R". The letter means the game is
// playable without color.
func (r *Renderer) Peg(c code.Color) string {
	return symPeg + string(c.Abbrev())
}

// Code returns all 5 pegs of c separated by spaces: "●R ●G ●Y ●B ●K".
func (r *Renderer) Code(c code.Code) string {
	pegs := make([]string, 0, code.Length)
	// Go note: ranging over an array gives each index and a copy of each
	// element, just as it does for a slice.
	for _, color := range c {
		pegs = append(pegs, r.Peg(color))
	}
	return strings.Join(pegs, " ")
}

// Feedback returns the result of a guess as exactly 5 symbols: one ● per
// exact match, then one △ per partial match, then · for the rest.
func (r *Renderer) Feedback(f code.Feedback) string {
	empty := code.Length - f.Exact - f.Partial
	return strings.Repeat(symExact, f.Exact) +
		strings.Repeat(symPartial, f.Partial) +
		strings.Repeat(symEmpty, empty)
}

// Board draws the full game screen for a round: a header with the mode,
// round number, roles and (in a match) scores; every guess so far with its
// feedback; and a legend.
func (r *Renderer) Board(info game.RoundInfo, turns []game.Turn) {
	var b strings.Builder

	b.WriteString("\n")
	fmt.Fprintf(&b, " CODEBREAKER · %s · Round %d\n", modeTitle(info.Mode), info.Number)
	fmt.Fprintf(&b, " Maker: %s · Breaker: %s\n", seatLabel(info.Maker), seatLabel(info.Breaker))
	if info.Mode == game.ModeMatch {
		fmt.Fprintf(&b, " Score: %s\n", scoreLine(info.Maker, info.Breaker))
	}

	b.WriteString(border("┌", "┬", "┐"))
	// Go note: in a format string, %-*s pads a string on the right to a
	// width given as an argument, here colGuess-1. fmt counts width in
	// runes (characters), not bytes, so the ● and △ symbols, which take 3
	// bytes each in UTF-8, still line up.
	fmt.Fprintf(&b, "│ Row │ %-*s│ %-*s│\n", colGuess-1, "Guess", colResult-1, "Result")
	b.WriteString(border("├", "┼", "┤"))
	for _, t := range turns {
		fmt.Fprintf(&b, "│ %3d │ %-*s│ %-*s│\n",
			t.Row, colGuess-1, r.Code(t.Guess), colResult-1, r.Feedback(t.Feedback))
	}
	b.WriteString(border("└", "┴", "┘"))

	fmt.Fprintf(&b, " %s right color, right spot  %s right color, wrong spot\n", symExact, symPartial)
	b.WriteString(" " + colorKey() + "\n")

	r.write(b.String())
}

// RoundStarted implements game.Observer. It forgets the previous round's
// turns and draws an empty board.
func (r *Renderer) RoundStarted(info game.RoundInfo) {
	r.mode = info.Mode
	// Go note: setting a slice to nil empties it. Appending to a nil slice
	// works fine, so no make is needed. Reusing the old slice with
	// r.turns[:0] would also work, but nil is clearer.
	r.turns = nil
	r.Board(info, nil)
}

// TurnPlayed implements game.Observer. It records the turn and redraws the
// board with every row so far.
func (r *Renderer) TurnPlayed(info game.RoundInfo, turn game.Turn) {
	r.turns = append(r.turns, turn)
	r.Board(info, r.turns)
}

// RoundEnded implements game.Observer. It reveals the secret and shows the
// rows used and the points. In a match it also shows the updated scores;
// in a Single game it says what the bracket would award (D13).
func (r *Renderer) RoundEnded(res game.RoundResult, seats [2]game.Seat) {
	breaker := seats[res.BreakerIdx].Name
	var b strings.Builder

	b.WriteString("\n")
	if res.Solved {
		fmt.Fprintf(&b, " Round %d over. %s cracked the code in %d %s!\n",
			res.Number, breaker, res.Rows, plural(res.Rows, "row"))
	} else {
		fmt.Fprintf(&b, " Round %d over. %s didn't crack the code in %d %s.\n",
			res.Number, breaker, res.Rows, plural(res.Rows, "row"))
	}
	fmt.Fprintf(&b, " Secret: %s\n", r.Code(res.Secret))

	switch {
	case !res.Solved:
		b.WriteString(" No points this round.\n")
	case r.mode == game.ModeSingle:
		lo, hi := bracket(res.Rows)
		fmt.Fprintf(&b, " In a match that would score %d points (solved in rows %d–%d).\n",
			res.Points, lo, hi)
	default:
		lo, hi := bracket(res.Rows)
		fmt.Fprintf(&b, " %s scores %d points (solved in rows %d–%d).\n",
			breaker, res.Points, lo, hi)
	}
	if r.mode == game.ModeMatch {
		fmt.Fprintf(&b, " Score: %s\n", scoreLine(seats[0], seats[1]))
	}

	r.write(b.String())
}

// MatchEnded implements game.Observer. It shows the winner and final
// scores, or says the game was stopped early (D16).
func (r *Renderer) MatchEnded(st game.Standings) {
	var b strings.Builder

	b.WriteString("\n")
	rounds := len(st.Results)
	switch {
	case r.mode == game.ModeSingle && rounds == 0:
		b.WriteString(" Game stopped before the code was cracked.\n")
	case r.mode == game.ModeSingle:
		b.WriteString(" Game over.\n")
	case st.Winner >= 0:
		winner := st.Seats[st.Winner]
		fmt.Fprintf(&b, " %s wins the match with %d points after %d %s!\n",
			winner.Name, winner.Score, rounds, plural(rounds, "round"))
		fmt.Fprintf(&b, " Final score: %s\n", scoreLine(st.Seats[0], st.Seats[1]))
	default:
		fmt.Fprintf(&b, " Match stopped after %d %s.\n", rounds, plural(rounds, "round"))
		fmt.Fprintf(&b, " Score: %s\n", scoreLine(st.Seats[0], st.Seats[1]))
	}

	r.write(b.String())
}

// bracket returns the range of rows that score the same points as solving
// in rows, such as 5 and 6 for rows = 5.
//
// It works the range out from game.Points instead of repeating the scoring
// table, so the screen can never disagree with the rules.
func bracket(rows int) (lo, hi int) {
	p := game.Points(rows, true)
	lo, hi = rows, rows
	for lo > 1 && game.Points(lo-1, true) == p {
		lo--
	}
	for hi < game.MaxRows && game.Points(hi+1, true) == p {
		hi++
	}
	// Go note: lo and hi are "named results", declared in the function
	// signature. They're ordinary variables, and a bare `return` returns
	// their current values. Here they also document what the two ints
	// mean.
	return
}

// modeTitle returns the mode as a screen heading: "Match" or "Single game".
func modeTitle(m game.Mode) string {
	if m == game.ModeSingle {
		return "Single game"
	}
	return "Match"
}

// seatLabel names a player with their scoring color: "Blue Tom".
func seatLabel(s game.Seat) string {
	return fmt.Sprintf("%v %s", s.Peg, s.Name)
}

// scoreLine shows two players' scores on the 30-point track:
// "Blue Tom 18/30 · Red Computer 10/30". Blue is always listed first, so
// the scores stay in the same place on every screen, whoever is Maker.
func scoreLine(a, b game.Seat) string {
	if a.Peg > b.Peg {
		// Go note: Go can swap two variables in one assignment. Both
		// right-hand values are read before either variable is set.
		a, b = b, a
	}
	return fmt.Sprintf("%s %d/%d · %s %d/%d",
		seatLabel(a), a.Score, game.TargetScore, seatLabel(b), b.Score, game.TargetScore)
}

// border draws one horizontal line of the board using the given corner and
// junction characters.
func border(left, mid, right string) string {
	return left + strings.Repeat("─", colRow) + mid +
		strings.Repeat("─", colGuess) + mid +
		strings.Repeat("─", colResult) + right + "\n"
}

// plural returns word, with an "s" added unless n is 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// colorKey lists every color with its letter: "K=Black W=White ...". It's
// built from code.AllColors, so it always matches the real palette.
func colorKey() string {
	parts := make([]string, 0, code.NumColors)
	for _, c := range code.AllColors() {
		parts = append(parts, fmt.Sprintf("%c=%s", c.Abbrev(), c.Name()))
	}
	return strings.Join(parts, " ")
}

// colorLetters lists every color letter: "K W R B G Y P O".
func colorLetters() string {
	letters := make([]string, 0, code.NumColors)
	for _, c := range code.AllColors() {
		letters = append(letters, string(c.Abbrev()))
	}
	return strings.Join(letters, " ")
}

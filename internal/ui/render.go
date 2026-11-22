package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
)

// Board column widths, in characters, not counting the borders. The guess
// column fits 5 pegs of 2 characters with spaces between and a space
// either side: 1 + 14 + 1 = 16. The result column fits 5 symbols with
// spaces between, the same way: 1 + 9 + 1 = 11.
const (
	colRow    = 5
	colGuess  = 16
	colResult = 11
)

// Renderer draws the game on an io.Writer.
//
// It remembers the current round's mode and the turns played so far,
// because the Observer events only carry the newest turn and the board
// shows them all.
type Renderer struct {
	w     io.Writer
	tty   bool           // w is a terminal: redraw in place, clear for handoffs
	color bool           // use ANSI colors
	sym   symbols        // the characters to draw with (Unicode or ASCII)
	mode  game.Mode      // mode of the latest round, used by the summaries
	info  game.RoundInfo // the current round, for Redraw
	turns []game.Turn    // turns played in the current round, oldest first
}

// Go note: a compile-time check that *Renderer satisfies game.Observer, so
// it can be passed straight to game.NewMatch.
var _ game.Observer = (*Renderer)(nil)

// NewRenderer returns a Renderer that writes to w, styled as o says. It
// checks once whether w is a terminal and whether color is allowed (see
// Options).
func NewRenderer(w io.Writer, o Options) *Renderer {
	tty, color, sym := newStyle(w, o)
	return &Renderer{w: w, tty: tty, color: color, sym: sym}
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

// Prompt formats text as a prompt, with a space before it and the prompt
// arrow after it: " Row 3/12 ▸ ". Players and the app build every prompt
// with it, so prompts follow the symbol set too.
func (r *Renderer) Prompt(text string) string {
	return " " + text + " " + r.sym.arrow + " "
}

// Range formats a range of numbers, such as "1–3", with the symbol set's
// dash.
func (r *Renderer) Range(lo, hi int) string {
	return fmt.Sprintf("%d%s%d", lo, r.sym.dash, hi)
}

// Peg returns one code peg as it appears on screen: a peg symbol followed
// by the color's letter, such as "●R", in that color when color is on.
// The letter means the game is playable without color.
func (r *Renderer) Peg(c code.Color) string {
	return r.paint(pegColors[c], r.sym.peg+string(c.Abbrev()))
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

// Feedback returns the result of a guess as exactly 5 symbols separated by
// spaces: one ● per exact match (bold), then one △ per partial match
// (yellow), then a dim · for the rest. The order says nothing about which
// pegs they mean.
func (r *Renderer) Feedback(f code.Feedback) string {
	return r.Marks(f.Marks())
}

// Marks returns easy-mode feedback: the same symbols as Feedback, but each
// one in the position of the guess peg it scores, so "· ● · · △" means the
// 2nd peg is in place and the 5th is the right color in the wrong spot.
//
// Feedback is built on Marks too, by sorting the marks first, so both
// always look alike.
func (r *Renderer) Marks(marks [code.Length]code.Mark) string {
	parts := make([]string, 0, code.Length)
	for _, m := range marks {
		switch m {
		case code.MarkExact:
			parts = append(parts, r.paint(sgrBold, r.sym.exact))
		case code.MarkPartial:
			parts = append(parts, r.paint(sgrYellow, r.sym.partial))
		default:
			parts = append(parts, r.paint(sgrDim, r.sym.empty))
		}
	}
	return strings.Join(parts, " ")
}

// result draws a turn's feedback the way the round asks for: by position
// in easy mode, as sorted counts otherwise.
func (r *Renderer) result(info game.RoundInfo, t game.Turn) string {
	if info.Easy {
		return r.Marks(t.Marks)
	}
	return r.Feedback(t.Feedback)
}

// Board draws the full game screen for a round: a header with the mode,
// round number, roles and (in a match) scores; every guess so far with its
// feedback; and a legend.
func (r *Renderer) Board(info game.RoundInfo, turns []game.Turn) {
	var b strings.Builder
	dot := r.sym.dot

	b.WriteString("\n")
	fmt.Fprintf(&b, " CODEBREAKER %s %s %s Round %d\n", dot, modeTitle(info.Mode), dot, info.Number)
	fmt.Fprintf(&b, " Maker: %s %s Breaker: %s\n", r.seatLabel(info.Maker), dot, r.seatLabel(info.Breaker))
	if info.Mode == game.ModeMatch {
		fmt.Fprintf(&b, " Score: %s\n", r.scoreLine(info.Maker, info.Breaker))
	}

	b.WriteString(r.border(r.sym.tl, r.sym.tm, r.sym.tr))
	r.row(&b, " Row ", "Guess", "Result")
	b.WriteString(r.border(r.sym.ml, r.sym.mm, r.sym.mr))
	for _, t := range turns {
		r.row(&b, fmt.Sprintf(" %3d ", t.Row), r.Code(t.Guess), r.result(info, t))
	}
	b.WriteString(r.border(r.sym.bl, r.sym.bm, r.sym.br))

	fmt.Fprintf(&b, " %s right color, right spot  %s right color, wrong spot\n",
		r.paint(sgrBold, r.sym.exact), r.paint(sgrYellow, r.sym.partial))
	if info.Easy {
		b.WriteString(" Easy mode: each symbol is in the same spot as the peg it scores.\n")
	}
	b.WriteString(" " + r.colorKey() + "\n")

	r.write(b.String())
}

// row writes one line of the board: the row cell as given, and the guess
// and result cells padded to their columns' widths.
func (r *Renderer) row(b *strings.Builder, rowCell, guess, result string) {
	v := r.sym.v
	b.WriteString(v + rowCell + v + " " + padRight(guess, colGuess-1) +
		v + " " + padRight(result, colResult-1) + v + "\n")
}

// RoundStarted implements game.Observer. It forgets the previous round's
// turns and draws an empty board.
//
// It doesn't clear the screen, even in a terminal, so the previous round's
// summary stays readable until the player answers the next prompt.
func (r *Renderer) RoundStarted(info game.RoundInfo) {
	r.mode = info.Mode
	r.info = info
	// Go note: setting a slice to nil empties it. Appending to a nil slice
	// works fine, so no make is needed. Reusing the old slice with
	// r.turns[:0] would also work, but nil is clearer.
	r.turns = nil
	r.Board(info, nil)
}

// TurnPlayed implements game.Observer. It records the turn and draws the
// board with every row so far. In a terminal it first clears the screen,
// so the board stays in one place rather than scrolling.
func (r *Renderer) TurnPlayed(info game.RoundInfo, turn game.Turn) {
	r.turns = append(r.turns, turn)
	if r.tty {
		r.write(ansiHome + ansiClearScreen)
	}
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
		fmt.Fprintf(&b, " In a match that would score %d points (solved in rows %s).\n",
			res.Points, r.Range(game.Bracket(res.Rows)))
	default:
		fmt.Fprintf(&b, " %s scores %d points (solved in rows %s).\n",
			breaker, res.Points, r.Range(game.Bracket(res.Rows)))
	}
	if r.mode == game.ModeMatch {
		fmt.Fprintf(&b, " Score: %s\n", r.scoreLine(seats[0], seats[1]))
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
		fmt.Fprintf(&b, " Final score: %s\n", r.scoreLine(st.Seats[0], st.Seats[1]))
	default:
		fmt.Fprintf(&b, " Match stopped after %d %s.\n", rounds, plural(rounds, "round"))
		fmt.Fprintf(&b, " Score: %s\n", r.scoreLine(st.Seats[0], st.Seats[1]))
	}

	r.write(b.String())
}

// modeTitle returns the mode as a screen heading: "Match" or "Single game".
func modeTitle(m game.Mode) string {
	if m == game.ModeSingle {
		return "Single game"
	}
	return "Match"
}

// seatLabel names a player with their scoring color: "Blue Tom", with the
// color word in that color when color is on.
func (r *Renderer) seatLabel(s game.Seat) string {
	return r.paint(scoringPegColor(s.Peg), s.Peg.String()) + " " + s.Name
}

// scoreLine shows two players' scores on the 30-point track:
// "Blue Tom 18/30 · Red Computer 10/30". Blue is always listed first, so
// the scores stay in the same place on every screen, whoever is Maker.
func (r *Renderer) scoreLine(a, b game.Seat) string {
	if a.Peg > b.Peg {
		// Go note: Go can swap two variables in one assignment. Both
		// right-hand values are read before either variable is set.
		a, b = b, a
	}
	return fmt.Sprintf("%s %d/%d %s %s %d/%d",
		r.seatLabel(a), a.Score, game.TargetScore, r.sym.dot, r.seatLabel(b), b.Score, game.TargetScore)
}

// border draws one horizontal line of the board using the given corner and
// junction characters.
func (r *Renderer) border(left, mid, right string) string {
	h := r.sym.hr
	return left + strings.Repeat(h, colRow) + mid +
		strings.Repeat(h, colGuess) + mid +
		strings.Repeat(h, colResult) + right + "\n"
}

// plural returns word, with an "s" added unless n is 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// colorKey lists every color with its letter: "K=Black W=White ...", each
// letter in its own color when color is on. It's built from
// code.AllColors, so it always matches the real palette.
func (r *Renderer) colorKey() string {
	parts := make([]string, 0, code.NumColors)
	for _, c := range code.AllColors() {
		parts = append(parts, r.paint(pegColors[c], string(c.Abbrev()))+"="+c.Name())
	}
	return strings.Join(parts, " ")
}

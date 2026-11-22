package web

import (
	"fmt"

	"codebreaker/internal/game"
)

// Board keeps the current round's board, so a View of it can be built at
// any time. It's the web version of the terminal's ui.Renderer: a Match
// tells it what happens, as its game.Observer, and it remembers.
//
// Unlike the terminal Renderer, Board draws nothing itself. Building the
// View, and deciding when to send it, is left to the code that owns the
// Board, which also knows what input is expected.
type Board struct {
	info      game.RoundInfo
	turns     []game.Turn
	result    *game.RoundResult // the latest finished round, until the next starts
	standings *game.Standings   // set when the match ends
}

// Go note: a compile-time check that *Board satisfies game.Observer.
var _ game.Observer = (*Board)(nil)

// NewBoard returns an empty board.
func NewBoard() *Board { return &Board{} }

// RoundStarted implements game.Observer. It forgets the previous round.
func (b *Board) RoundStarted(info game.RoundInfo) {
	b.info = info
	b.turns = nil
	b.result = nil
}

// TurnPlayed implements game.Observer. It records the turn.
func (b *Board) TurnPlayed(info game.RoundInfo, turn game.Turn) {
	b.info = info
	b.turns = append(b.turns, turn)
}

// RoundEnded implements game.Observer. It keeps the result for the
// summary screen, and the players' new scores for the board's header.
func (b *Board) RoundEnded(res game.RoundResult, seats [2]game.Seat) {
	b.result = &res
	b.info.Maker = seats[res.MakerIdx]
	b.info.Breaker = seats[res.BreakerIdx]
}

// MatchEnded implements game.Observer. It keeps the standings for the
// result screen.
func (b *Board) MatchEnded(st game.Standings) {
	b.standings = &st
}

// Result returns the round that just finished, and false if none has
// finished since the last round started.
func (b *Board) Result() (game.RoundResult, bool) {
	if b.result == nil {
		return game.RoundResult{}, false
	}
	return *b.result, true
}

// Standings returns the final standings, and false until the match ends.
func (b *Board) Standings() (game.Standings, bool) {
	if b.standings == nil {
		return game.Standings{}, false
	}
	return *b.standings, true
}

// View describes the board for the page, with draft as the row being
// built. Pass nil when no one is building a row.
func (b *Board) View(draft *Draft) *BoardView {
	v := &BoardView{
		Title:   fmt.Sprintf("%s · Round %d", modeTitle(b.info.Mode), b.info.Number),
		Maker:   seatView(b.info.Maker),
		Breaker: seatView(b.info.Breaker),
		Rows:    make([]RowView, 0, len(b.turns)),
		MaxRows: b.info.MaxRows,
		Easy:    b.info.Easy,
	}
	if b.info.Mode == game.ModeMatch {
		blue, red := v.Maker, v.Breaker
		if blue.Peg != "blue" {
			blue, red = red, blue
		}
		v.Scores = []SeatView{blue, red}
		v.Target = game.TargetScore
	}
	for _, t := range b.turns {
		marks := t.Feedback.Marks()
		if b.info.Easy {
			marks = t.Marks
		}
		v.Rows = append(v.Rows, RowView{Row: t.Row, Guess: pegViews(t.Guess), Feedback: markNames(marks)})
	}
	if draft != nil {
		v.Draft = draft.Pegs()
	}
	if b.result != nil {
		v.Secret = pegViews(b.result.Secret)
	}
	return v
}

// modeTitle returns the mode as a heading: "Match" or "Single game".
func modeTitle(m game.Mode) string {
	if m == game.ModeSingle {
		return "Single game"
	}
	return "Match"
}

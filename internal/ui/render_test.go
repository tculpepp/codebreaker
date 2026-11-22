package ui

import (
	"strings"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// mustParse turns a code string into a code.Code, failing the test on a
// typo in the test itself.
func mustParse(t *testing.T, s string) code.Code {
	t.Helper()
	c, err := code.Parse(s)
	if err != nil {
		t.Fatalf("bad code %q in test: %v", s, err)
	}
	return c
}

// Seats and round info shared by the tests. Players are nil because the
// renderer only reads names, pegs and scores.
var (
	tom      = game.Seat{Name: "Tom", Peg: game.PegBlue}
	computer = game.Seat{Name: "Computer", Peg: game.PegRed}

	singleInfo = game.RoundInfo{Number: 1, Mode: game.ModeSingle, MaxRows: game.MaxRows,
		Maker: computer, Breaker: tom}
)

// newTestRenderer returns a renderer writing into a builder the test can
// read.
func newTestRenderer() (*Renderer, *strings.Builder) {
	var out strings.Builder
	return NewRenderer(&out), &out
}

// TestPegCodeFeedback checks the small building blocks every screen uses.
// Each peg shows its letter so the game is playable without color, and
// feedback is always 5 symbols: exact first, then partial, then empty.
func TestPegCodeFeedback(t *testing.T) {
	r, _ := newTestRenderer()
	if got, want := r.Peg(code.Pink), "●P"; got != want {
		t.Errorf("Peg(Pink) = %q, want %q", got, want)
	}
	if got, want := r.Code(mustParse(t, "RGYBK")), "●R ●G ●Y ●B ●K"; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	feedback := []struct {
		fb   code.Feedback
		want string
	}{
		{code.Feedback{}, "·····"},
		{code.Feedback{Exact: 1, Partial: 2}, "●△△··"},
		{code.Feedback{Partial: 5}, "△△△△△"},
		{code.Feedback{Exact: 5}, "●●●●●"},
		{code.Feedback{Exact: 3, Partial: 2}, "●●●△△"},
	}
	for _, tt := range feedback {
		if got := r.Feedback(tt.fb); got != tt.want {
			t.Errorf("Feedback(%+v) = %q, want %q", tt.fb, got, tt.want)
		}
	}
}

// TestBracket checks the row ranges quoted in summaries and help. They are
// worked out from game.Points, so this also proves the two agree.
func TestBracket(t *testing.T) {
	tests := []struct {
		rows   int
		lo, hi int
	}{
		{1, 1, 4}, {4, 1, 4}, {5, 5, 6}, {6, 5, 6}, {7, 7, 8},
		{9, 9, 10}, {11, 11, 12}, {12, 11, 12},
	}
	for _, tt := range tests {
		if lo, hi := bracket(tt.rows); lo != tt.lo || hi != tt.hi {
			t.Errorf("bracket(%d) = %d–%d, want %d–%d", tt.rows, lo, hi, tt.lo, tt.hi)
		}
	}
}

// TestBoardEmpty is the screen at the start of a Single game, before any
// guess. There's no score line, because a Single game has no match total
// (D13).
func TestBoardEmpty(t *testing.T) {
	r, out := newTestRenderer()
	r.Board(singleInfo, nil)
	checkGolden(t, "board_empty", out.String())
}

// TestBoardMatchWithRows is a board mid-match: the header shows the round
// and both scores out of 30, and each played row shows its feedback.
func TestBoardMatchWithRows(t *testing.T) {
	r, out := newTestRenderer()
	maker, breaker := computer, tom
	maker.Score, breaker.Score = 10, 18
	info := game.RoundInfo{Number: 3, Mode: game.ModeMatch, MaxRows: game.MaxRows,
		Maker: maker, Breaker: breaker}
	turns := []game.Turn{
		{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Exact: 1, Partial: 2}},
		{Row: 2, Guess: mustParse(t, "WRPOG"), Feedback: code.Feedback{Exact: 2, Partial: 1}},
	}
	r.Board(info, turns)
	checkGolden(t, "board_match", out.String())
}

// TestObserverDrawsBoardEachTurn checks the Observer side: a new round
// draws an empty board, and each played turn redraws it with every row so
// far. A new round starts again from an empty board.
func TestObserverDrawsBoardEachTurn(t *testing.T) {
	r, out := newTestRenderer()
	t1 := game.Turn{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Partial: 3}}
	t2 := game.Turn{Row: 2, Guess: mustParse(t, "WPOGR"), Feedback: code.Feedback{Exact: 1, Partial: 1}}

	r.RoundStarted(singleInfo)
	r.TurnPlayed(singleInfo, t1)
	r.TurnPlayed(singleInfo, t2)
	if got := strings.Count(out.String(), "│   1 │"); got != 2 {
		t.Errorf("row 1 drawn %d times, want 2 (once per board after it)", got)
	}
	if got := strings.Count(out.String(), "│   2 │"); got != 1 {
		t.Errorf("row 2 drawn %d times, want 1", got)
	}

	out.Reset()
	r.RoundStarted(singleInfo)
	if strings.Contains(out.String(), "│   1 │") {
		t.Errorf("new round still shows old rows:\n%s", out.String())
	}
}

// roundEnded runs RoundStarted (to set the mode) and returns only what
// RoundEnded writes.
func roundEnded(info game.RoundInfo, res game.RoundResult, seats [2]game.Seat) string {
	r, out := newTestRenderer()
	r.RoundStarted(info)
	out.Reset()
	r.RoundEnded(res, seats)
	return out.String()
}

// TestRoundSummaries checks the end-of-round screen in each situation:
// the secret is revealed, with the rows used and the points. In a match
// the updated scores follow; in a Single game the points are what the
// bracket would award (D13).
func TestRoundSummaries(t *testing.T) {
	secret := mustParse(t, "WPORG")
	matchInfo := singleInfo
	matchInfo.Mode = game.ModeMatch
	scored := [2]game.Seat{tom, computer}
	scored[0].Score = 10

	tests := []struct {
		name  string
		info  game.RoundInfo
		res   game.RoundResult
		seats [2]game.Seat
	}{
		{name: "round_match_solved", info: matchInfo, seats: scored,
			res: game.RoundResult{Number: 1, MakerIdx: 1, BreakerIdx: 0, Rows: 5, Solved: true, Secret: secret, Points: 10}},
		{name: "round_match_unsolved", info: matchInfo, seats: [2]game.Seat{tom, computer},
			res: game.RoundResult{Number: 2, MakerIdx: 1, BreakerIdx: 0, Rows: 12, Secret: secret}},
		{name: "round_single_solved_one_row", info: singleInfo, seats: [2]game.Seat{tom, computer},
			res: game.RoundResult{Number: 1, MakerIdx: 1, BreakerIdx: 0, Rows: 1, Solved: true, Secret: secret, Points: 12}},
		{name: "round_single_unsolved", info: singleInfo, seats: [2]game.Seat{tom, computer},
			res: game.RoundResult{Number: 1, MakerIdx: 1, BreakerIdx: 0, Rows: 12, Secret: secret}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkGolden(t, tt.name, roundEnded(tt.info, tt.res, tt.seats))
		})
	}
}

// TestMatchSummaries checks the final screen: a winner, a match stopped
// early (quit or Ctrl-C), and the two Single-game endings.
func TestMatchSummaries(t *testing.T) {
	won := [2]game.Seat{tom, computer}
	won[0].Score, won[1].Score = 32, 18
	oneRound := []game.RoundResult{{Number: 1}}
	fiveRounds := make([]game.RoundResult, 5)

	tests := []struct {
		name string
		mode game.Mode
		st   game.Standings
	}{
		{name: "match_won", mode: game.ModeMatch, st: game.Standings{Seats: won, Results: fiveRounds, Winner: 0}},
		{name: "match_stopped", mode: game.ModeMatch, st: game.Standings{Seats: [2]game.Seat{tom, computer}, Results: oneRound, Winner: -1}},
		{name: "single_over", mode: game.ModeSingle, st: game.Standings{Seats: [2]game.Seat{tom, computer}, Results: oneRound, Winner: -1}},
		{name: "single_stopped", mode: game.ModeSingle, st: game.Standings{Seats: [2]game.Seat{tom, computer}, Winner: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out := newTestRenderer()
			info := singleInfo
			info.Mode = tt.mode
			r.RoundStarted(info)
			out.Reset()
			r.MatchEnded(tt.st)
			checkGolden(t, tt.name, out.String())
		})
	}
}

// TestNoRoundStartedMeansMatch checks the renderer's default: before any
// round has started it assumes a full match, the zero value of game.Mode.
func TestNoRoundStartedMeansMatch(t *testing.T) {
	r, out := newTestRenderer()
	r.MatchEnded(game.Standings{Seats: [2]game.Seat{tom, computer}, Winner: -1})
	if !strings.Contains(out.String(), "Match stopped after 0 rounds") {
		t.Errorf("output = %q, want a stopped-match summary", out.String())
	}
}

// TestRendererIsAnObserver documents, in a test, what the compile-time
// check in render.go guarantees: a *Renderer can be handed to
// game.NewMatch as its Observer.
func TestRendererIsAnObserver(t *testing.T) {
	var obs game.Observer = NewRenderer(&strings.Builder{})
	_ = obs
}

// TestSetup checks the screen shown before play starts: who is playing,
// what wins, and who makes the first code, saying so when a coin flip
// decided it (requirements.md "Menu System": show the coin-flip result).
func TestSetup(t *testing.T) {
	seats := [2]game.Seat{tom, computer}
	tests := []struct {
		name     string
		mode     game.Mode
		makerIdx int
		coinFlip bool
	}{
		{name: "setup_match_coin_flip", mode: game.ModeMatch, makerIdx: 1, coinFlip: true},
		{name: "setup_single_chosen", mode: game.ModeSingle, makerIdx: 0},
		{name: "setup_single_coin_flip", mode: game.ModeSingle, makerIdx: 0, coinFlip: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out := newTestRenderer()
			r.Setup(tt.mode, seats, tt.makerIdx, tt.coinFlip)
			checkGolden(t, tt.name, out.String())
		})
	}
}

package web

import (
	"encoding/json"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
)

// playTurns records a round on b: it starts, then each guess is scored
// against secret, as game.Round would.
func playTurns(t *testing.T, b *Board, info game.RoundInfo, secret string, guesses ...string) {
	t.Helper()
	b.RoundStarted(info)
	s := mustParse(t, secret)
	for i, g := range guesses {
		guess := mustParse(t, g)
		b.TurnPlayed(info, game.Turn{Row: i + 1, Guess: guess,
			Feedback: code.Score(s, guess), Marks: code.Marks(s, guess)})
	}
}

// TestBoardViewMatch checks a board mid-match, turned into the JSON the
// page receives. The golden file is the contract with render.js: the
// header, both seats with their peg colors, the scores (Blue first), the
// rows with sorted feedback, and the draft.
func TestBoardViewMatch(t *testing.T) {
	maker, breaker := computer, tom
	maker.Score, breaker.Score = 10, 18
	info := game.RoundInfo{Number: 3, Mode: game.ModeMatch, MaxRows: game.MaxRows,
		Maker: maker, Breaker: breaker}
	b := NewBoard()
	playTurns(t, b, info, "RGYBK", "WGPOR", "KGRBW")

	var d Draft
	d.Add(code.Black)
	view := View{Screen: ScreenBoard, Board: b.View(&d), Input: InputView{Palette: true, Undo: true, Submit: true}}
	got, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "board_match", got)
}

// TestBoardViewEasy checks that on Easy each row's feedback is by
// position, and on Normal it's sorted, for the same guesses.
func TestBoardViewEasy(t *testing.T) {
	info := game.RoundInfo{Number: 1, Mode: game.ModeSingle, MaxRows: game.MaxRows,
		Maker: computer, Breaker: tom}
	for _, tt := range []struct {
		easy bool
		want []string
	}{
		{false, []string{"exact", "partial", "none", "none", "none"}},
		{true, []string{"none", "exact", "none", "none", "partial"}},
	} {
		info.Easy = tt.easy
		b := NewBoard()
		playTurns(t, b, info, "RGYBK", "WGPOR")
		v := b.View(nil)
		if v.Easy != tt.easy || len(v.Rows) != 1 {
			t.Fatalf("easy=%v: view = %+v", tt.easy, v)
		}
		for i, w := range tt.want {
			if v.Rows[0].Feedback[i] != w {
				t.Errorf("easy=%v: feedback = %v, want %v", tt.easy, v.Rows[0].Feedback, tt.want)
				break
			}
		}
		if v.Scores != nil || v.Draft != nil {
			t.Errorf("single game with no draft: scores %v, draft %v; want neither", v.Scores, v.Draft)
		}
		if v.Title != "Single game · Round 1" {
			t.Errorf("title = %q", v.Title)
		}
	}
}

// TestBoardNewRoundStartsEmpty checks that a new round forgets the last
// round's rows, and that the board keeps each round's result and the
// final standings for the summary screens.
func TestBoardNewRoundStartsEmpty(t *testing.T) {
	info := game.RoundInfo{Number: 1, Mode: game.ModeMatch, MaxRows: game.MaxRows,
		Maker: computer, Breaker: tom}
	b := NewBoard()
	if _, ok := b.Result(); ok {
		t.Error("Result() before any round reported a result")
	}
	if _, ok := b.Standings(); ok {
		t.Error("Standings() before the end reported standings")
	}

	playTurns(t, b, info, "RGYBK", "WGPOR")
	res := game.RoundResult{Number: 1, Rows: 1}
	b.RoundEnded(res, [2]game.Seat{tom, computer})
	if got, ok := b.Result(); !ok || got.Number != 1 {
		t.Errorf("Result() = %+v, %v", got, ok)
	}

	info.Number = 2
	b.RoundStarted(info)
	if v := b.View(nil); len(v.Rows) != 0 || v.Title != "Match · Round 2" {
		t.Errorf("new round view = %+v, want no rows", v)
	}
	if _, ok := b.Result(); ok {
		t.Error("Result() still reported round 1 after round 2 started")
	}

	b.MatchEnded(game.Standings{Winner: 1})
	if st, ok := b.Standings(); !ok || st.Winner != 1 {
		t.Errorf("Standings() = %+v, %v", st, ok)
	}
}

// TestBoardIsAnObserver checks at compile time and at run time that a
// Board can be handed to game.NewMatch.
func TestBoardIsAnObserver(t *testing.T) {
	var _ game.Observer = NewBoard()
}

// TestSeatView checks how players appear in a View: name, peg color as a
// lowercase word the page uses as a CSS class, and score.
func TestSeatView(t *testing.T) {
	s := tom
	s.Score = 12
	if got, want := seatView(s), (SeatView{Name: "Tom", Peg: "blue", Score: 12}); got != want {
		t.Errorf("seatView = %+v, want %+v", got, want)
	}
	if got := seatView(computer).Peg; got != "red" {
		t.Errorf("computer's peg = %q, want red", got)
	}
}

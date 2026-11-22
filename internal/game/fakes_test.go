package game

import (
	"context"
	"fmt"
	"strings"

	"codebreaker/internal/code"
)

// This file holds test doubles: simple stand-ins for real players and for
// the screen, so Match can be tested without a keyboard or terminal.
//
// Go note: because Go interfaces are satisfied implicitly, any type with
// the right methods can stand in for a real player. No mocking library is
// needed; a small struct with hand-written methods is the idiomatic way.

// fakePlayer is a scripted Participant.
//
// As Code Maker it always returns secret. As Code Breaker it "knows" the
// opponent's secret (target) and solves it on a chosen row, guessing
// distinct wrong codes before that. That lets a test choose exactly how
// many points each round scores.
type fakePlayer struct {
	secret   code.Code   // what MakeSecret returns
	target   code.Code   // the opponent's secret, which this fake will "crack"
	solveAt  map[int]int // round number → row to solve on; a missing round means never solve
	makeErr  error       // if set, MakeSecret fails with it
	guessErr error       // if set, Guess fails with it

	repeatOn int                // if >0, on this row, re-send the row-1 guess (a buggy breaker)
	cancelOn int                // if >0, on this row, call cancel before guessing (Ctrl-C mid-round)
	cancel   context.CancelFunc // used with cancelOn
}

// Go note: this line is a compile-time check that *fakePlayer satisfies
// Participant. It creates a nil *fakePlayer, converts it to the interface,
// and throws it away (_). If a method were missing or had the wrong
// signature, the build would fail right here with a clear message.
var _ Participant = (*fakePlayer)(nil)

// MakeSecret returns the scripted secret or error.
func (f *fakePlayer) MakeSecret(ctx context.Context, info RoundInfo) (code.Code, error) {
	if f.makeErr != nil {
		return code.Code{}, f.makeErr
	}
	return f.secret, nil
}

// Guess returns the target on the scripted row, otherwise a wrong guess
// that's different on every row (so it never trips the repeat rule).
func (f *fakePlayer) Guess(ctx context.Context, info RoundInfo, history []Turn) (code.Code, error) {
	if f.guessErr != nil {
		return code.Code{}, f.guessErr
	}
	row := len(history) + 1
	if row == f.cancelOn {
		f.cancel()
	}
	if row == f.repeatOn {
		return history[0].Guess, nil
	}
	if row == f.solveAt[info.Number] {
		return f.target, nil
	}
	return wrongGuesses(f.target, row)[row-1], nil
}

// newPair makes two fake players, Alice (Blue) and Bob (Red), each aimed at
// the other's secret. Alice's secret is RGYBK and Bob's is WPORG.
func newPair() (alice, bob *fakePlayer, seats [2]Seat) {
	aliceSecret, _ := code.Parse("RGYBK")
	bobSecret, _ := code.Parse("WPORG")
	alice = &fakePlayer{secret: aliceSecret, target: bobSecret, solveAt: map[int]int{}}
	bob = &fakePlayer{secret: bobSecret, target: aliceSecret, solveAt: map[int]int{}}
	seats = [2]Seat{
		{Name: "Alice", Peg: PegBlue, Player: alice},
		{Name: "Bob", Peg: PegRed, Player: bob},
	}
	return alice, bob, seats
}

// recorder is a fake Observer that writes each event as a short line of
// text. Tests compare the lines to check what was reported, and in what
// order.
type recorder struct {
	events []string
}

var _ Observer = (*recorder)(nil)

func (r *recorder) RoundStarted(info RoundInfo) {
	r.add("start r%d maker=%s breaker=%s", info.Number, info.Maker.Name, info.Breaker.Name)
}

func (r *recorder) TurnPlayed(info RoundInfo, t Turn) {
	r.add("turn r%d row%d", info.Number, t.Row)
}

func (r *recorder) RoundEnded(res RoundResult, seats [2]Seat) {
	r.add("end r%d solved=%v rows=%d pts=%d score=%d-%d",
		res.Number, res.Solved, res.Rows, res.Points, seats[0].Score, seats[1].Score)
}

func (r *recorder) MatchEnded(st Standings) {
	r.add("match winner=%d", st.Winner)
}

// add formats one event and appends it.
//
// Go note: `args ...any` accepts any number of values of any type, just like
// fmt.Sprintf, and `args...` passes them straight on. `any` is an alias for
// `interface{}`, the empty interface that every type satisfies.
func (r *recorder) add(format string, args ...any) {
	r.events = append(r.events, fmt.Sprintf(format, args...))
}

// String joins the events one per line, for readable failure messages.
func (r *recorder) String() string { return strings.Join(r.events, "\n") }

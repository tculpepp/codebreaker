package game

import (
	"errors"
	"testing"

	"codebreaker/internal/code"
)

// wrongGuesses returns n different valid codes, none equal to secret. Tests
// use them to fill rows without solving the round.
func wrongGuesses(secret code.Code, n int) []code.Code {
	out := make([]code.Code, 0, n)
	for _, c := range code.All() {
		if len(out) == n {
			break
		}
		if c != secret {
			out = append(out, c)
		}
	}
	return out
}

// TestNewRoundRejectsInvalidSecret checks that a round can't start with a
// secret that breaks the rules, such as the zero Code (five Blacks).
func TestNewRoundRejectsInvalidSecret(t *testing.T) {
	_, err := NewRound(code.Code{})
	var dup *code.DuplicateColorError
	if !errors.As(err, &dup) {
		t.Fatalf("NewRound(zero code) error = %v, want *code.DuplicateColorError", err)
	}
}

// TestRoundSolved plays two wrong guesses and then the secret, checking row
// numbers, feedback, and the final status.
func TestRoundSolved(t *testing.T) {
	secret := mustParse(t, "RGYBK")
	r, err := NewRound(secret)
	if err != nil {
		t.Fatal(err)
	}

	if r.Status() != InProgress || r.RowsUsed() != 0 || r.RowsLeft() != MaxRows {
		t.Fatalf("new round: status=%v used=%d left=%d", r.Status(), r.RowsUsed(), r.RowsLeft())
	}

	turn, err := r.Submit(mustParse(t, "RYGWP"))
	if err != nil {
		t.Fatal(err)
	}
	want := Turn{Row: 1, Guess: mustParse(t, "RYGWP"), Feedback: code.Feedback{Exact: 1, Partial: 2},
		// Every turn also carries the easy-mode marks, one per position.
		Marks: [code.Length]code.Mark{code.MarkExact, code.MarkPartial, code.MarkPartial, code.MarkNone, code.MarkNone}}
	if turn != want {
		t.Errorf("turn 1 = %+v, want %+v", turn, want)
	}

	if _, err := r.Submit(mustParse(t, "WPORG")); err != nil {
		t.Fatal(err)
	}
	turn, err = r.Submit(secret)
	if err != nil {
		t.Fatal(err)
	}
	if turn.Row != 3 || !turn.Feedback.Solved() {
		t.Errorf("winning turn = %+v, want row 3 solved", turn)
	}
	if r.Status() != Solved {
		t.Errorf("status = %v, want Solved", r.Status())
	}
	if r.RowsUsed() != 3 || r.RowsLeft() != MaxRows-3 {
		t.Errorf("used=%d left=%d, want 3 and %d", r.RowsUsed(), r.RowsLeft(), MaxRows-3)
	}
}

// TestRoundOutOfRows checks the 12-row limit (D3): after 12 wrong guesses
// the round ends unsolved, and a 13th guess is refused.
func TestRoundOutOfRows(t *testing.T) {
	secret := mustParse(t, "RGYBK")
	r, _ := NewRound(secret)
	guesses := wrongGuesses(secret, MaxRows+1)

	for i, g := range guesses[:MaxRows] {
		if _, err := r.Submit(g); err != nil {
			t.Fatalf("guess %d: %v", i+1, err)
		}
		// The status must stay InProgress right up to the last row.
		if i < MaxRows-1 && r.Status() != InProgress {
			t.Fatalf("after guess %d status = %v, want InProgress", i+1, r.Status())
		}
	}
	if r.Status() != OutOfRows {
		t.Fatalf("status = %v, want OutOfRows", r.Status())
	}
	if _, err := r.Submit(guesses[MaxRows]); !errors.Is(err, ErrRoundOver) {
		t.Errorf("13th guess error = %v, want ErrRoundOver", err)
	}
}

// TestRoundSolvedOnLastRow checks the edge case where the code is cracked
// on row 12: that counts as Solved, not OutOfRows.
func TestRoundSolvedOnLastRow(t *testing.T) {
	secret := mustParse(t, "RGYBK")
	r, _ := NewRound(secret)
	for _, g := range wrongGuesses(secret, MaxRows-1) {
		r.Submit(g)
	}
	if _, err := r.Submit(secret); err != nil {
		t.Fatal(err)
	}
	if r.Status() != Solved {
		t.Errorf("status = %v, want Solved", r.Status())
	}
}

// TestRoundRejectsBadGuesses checks that an invalid or repeated guess is
// refused and does NOT use a row (requirements "Input Validation", D14).
func TestRoundRejectsBadGuesses(t *testing.T) {
	r, _ := NewRound(mustParse(t, "RGYBK"))
	first := mustParse(t, "WPORG")
	r.Submit(first)

	_, err := r.Submit(first)
	var rep *RepeatGuessError
	if !errors.As(err, &rep) || rep.Row != 1 {
		t.Errorf("repeat error = %v, want *RepeatGuessError for row 1", err)
	}

	_, err = r.Submit(code.Code{}) // five Blacks: invalid
	var dup *code.DuplicateColorError
	if !errors.As(err, &dup) {
		t.Errorf("invalid guess error = %v, want *code.DuplicateColorError", err)
	}

	if r.RowsUsed() != 1 {
		t.Errorf("RowsUsed = %d after two rejected guesses, want 1", r.RowsUsed())
	}
}

// TestRoundSecretHiddenUntilOver checks that the secret can't be read while
// the round is in progress. Code Breakers (including the computer) never get
// a way to peek.
func TestRoundSecretHiddenUntilOver(t *testing.T) {
	secret := mustParse(t, "RGYBK")
	r, _ := NewRound(secret)

	if s, ok := r.Secret(); ok || s != (code.Code{}) {
		t.Errorf("Secret() during play = (%v, %v), want zero Code and false", s, ok)
	}
	r.Submit(secret)
	if s, ok := r.Secret(); !ok || s != secret {
		t.Errorf("Secret() after solve = (%v, %v), want (%v, true)", s, ok, secret)
	}
}

// TestRoundAbandon checks quitting mid-round: the round ends, the secret
// can be revealed, and no more guesses are accepted.
func TestRoundAbandon(t *testing.T) {
	r, _ := NewRound(mustParse(t, "RGYBK"))
	r.Submit(mustParse(t, "WPORG"))
	r.Abandon()

	if r.Status() != Abandoned {
		t.Fatalf("status = %v, want Abandoned", r.Status())
	}
	if _, ok := r.Secret(); !ok {
		t.Error("Secret() not available after Abandon")
	}
	if _, err := r.Submit(mustParse(t, "KBYGR")); !errors.Is(err, ErrRoundOver) {
		t.Errorf("Submit after Abandon = %v, want ErrRoundOver", err)
	}

	// Abandoning a finished round must not change how it ended.
	r2, _ := NewRound(mustParse(t, "RGYBK"))
	r2.Submit(mustParse(t, "RGYBK"))
	r2.Abandon()
	if r2.Status() != Solved {
		t.Errorf("Abandon changed a Solved round to %v", r2.Status())
	}
}

// TestRoundTurnsReturnsCopy checks that callers can't rewrite history by
// changing the slice Turns returns.
func TestRoundTurnsReturnsCopy(t *testing.T) {
	r, _ := NewRound(mustParse(t, "RGYBK"))
	r.Submit(mustParse(t, "WPORG"))

	turns := r.Turns()
	turns[0].Feedback = code.Feedback{Exact: 5}

	if r.Turns()[0].Feedback.Exact == 5 {
		t.Error("changing the returned slice changed the round's history")
	}
}

// TestRoundStatusString checks the readable names used in messages.
func TestRoundStatusString(t *testing.T) {
	tests := map[RoundStatus]string{
		InProgress:      "in progress",
		Solved:          "solved",
		OutOfRows:       "out of rows",
		Abandoned:       "abandoned",
		RoundStatus(99): "RoundStatus(99)",
	}
	// Go note: ranging over a map visits the entries in random order. That
	// is fine here because each case is independent.
	for status, want := range tests {
		if got := status.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

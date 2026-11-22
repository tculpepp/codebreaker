package game

import (
	"reflect"
	"testing"

	"codebreaker/internal/code"
)

// mustParse turns "RGYBK" into a code.Code or stops the test. It is shared
// by every test file in this package.
//
// Go note: all _test.go files in a folder are compiled together, so a
// helper declared in one file can be used in the others.
func mustParse(t testing.TB, s string) code.Code {
	t.Helper()
	c, err := code.Parse(s)
	if err != nil {
		t.Fatalf("mustParse(%q): %v", s, err)
	}
	return c
}

// TestPoints checks every row against the box-rules bracket table
// (requirements.md "Scoring", D4). Each row is listed separately so a
// mistake at a bracket's edge (4→5, 6→7, ...) is caught.
func TestPoints(t *testing.T) {
	want := map[int]int{
		1: 12, 2: 12, 3: 12, 4: 12,
		5: 10, 6: 10,
		7: 8, 8: 8,
		9: 6, 10: 6,
		11: 4, 12: 4,
	}
	for rows := 1; rows <= MaxRows; rows++ {
		if got := Points(rows, true); got != want[rows] {
			t.Errorf("Points(%d, solved) = %d, want %d", rows, got, want[rows])
		}
	}
}

// TestPointsUnsolvedAndOutOfRange checks the zero cases: an unsolved
// round always scores 0 (D4), and impossible row counts also score 0
// rather than inventing points.
func TestPointsUnsolvedAndOutOfRange(t *testing.T) {
	tests := []struct {
		name   string
		rows   int
		solved bool
	}{
		{"unsolved after 12", 12, false},
		{"unsolved after 3 (abandoned)", 3, false},
		{"solved in 0 rows is impossible", 0, true},
		{"solved in 13 rows is impossible", 13, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Points(tt.rows, tt.solved); got != 0 {
				t.Errorf("Points(%d, %v) = %d, want 0", tt.rows, tt.solved, got)
			}
		})
	}
}

// TestCheckRepeat checks the repeated-guess rule (D14): a guess identical
// to an earlier one this round is rejected, with the row it was first used.
func TestCheckRepeat(t *testing.T) {
	history := []Turn{
		{Row: 1, Guess: mustParse(t, "RGYBK")},
		{Row: 2, Guess: mustParse(t, "WPORG")},
	}

	if err := CheckRepeat(history, mustParse(t, "KBYGR")); err != nil {
		t.Errorf("new guess rejected: %v", err)
	}
	if err := CheckRepeat(nil, mustParse(t, "RGYBK")); err != nil {
		t.Errorf("first guess of a round rejected: %v", err)
	}

	err := CheckRepeat(history, mustParse(t, "WPORG"))
	want := &RepeatGuessError{Guess: mustParse(t, "WPORG"), Row: 2}
	if !reflect.DeepEqual(err, want) {
		t.Errorf("CheckRepeat error = %#v, want %#v", err, want)
	}
}

// TestRepeatGuessErrorMessage pins the developer-facing message.
func TestRepeatGuessErrorMessage(t *testing.T) {
	err := &RepeatGuessError{Guess: mustParse(t, "RGYBK"), Row: 2}
	if got, want := err.Error(), "RGYBK was already guessed in row 2"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestBracket checks the row ranges both front ends quote in summaries and
// help. They are worked out from Points, so this also proves the two
// agree.
func TestBracket(t *testing.T) {
	tests := []struct {
		rows   int
		lo, hi int
	}{
		{1, 1, 4}, {4, 1, 4}, {5, 5, 6}, {6, 5, 6}, {7, 7, 8},
		{9, 9, 10}, {11, 11, 12}, {12, 11, 12},
	}
	for _, tt := range tests {
		if lo, hi := Bracket(tt.rows); lo != tt.lo || hi != tt.hi {
			t.Errorf("Bracket(%d) = %d–%d, want %d–%d", tt.rows, lo, hi, tt.lo, tt.hi)
		}
	}
}

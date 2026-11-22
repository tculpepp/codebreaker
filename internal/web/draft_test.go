package web

import (
	"errors"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/message"
)

// TestDraftBuildsACode adds five colors, undoes one, and finishes the
// code: the row being built on the board.
func TestDraftBuildsACode(t *testing.T) {
	var d Draft // the zero Draft is empty and ready to use
	for _, c := range []code.Color{code.Red, code.Green, code.Yellow, code.Blue, code.Pink} {
		if err := d.Add(c); err != nil {
			t.Fatalf("Add(%v): %v", c, err)
		}
	}
	if !d.Undo() || d.Len() != 4 {
		t.Fatalf("after Undo, Len = %d, want 4", d.Len())
	}
	if err := d.Add(code.Black); err != nil {
		t.Fatal(err)
	}
	got, err := d.Code()
	if err != nil || got != mustParse(t, "RGYBK") {
		t.Errorf("Code() = %v, %v; want RGYBK, nil", got, err)
	}

	d.Clear()
	if d.Len() != 0 || d.Undo() {
		t.Errorf("after Clear: Len = %d, Undo worked; want an empty draft", d.Len())
	}
}

// TestDraftRefusals checks the mistakes a draft refuses, each with a
// friendly message: a color already used, a full row, an unfinished row
// and an invalid color.
func TestDraftRefusals(t *testing.T) {
	var d Draft
	d.Add(code.Red)
	d.Add(code.Green)

	err := d.Add(code.Red)
	var dup *code.DuplicateColorError
	if !errors.As(err, &dup) || dup.First != 1 || dup.Second != 3 || d.Len() != 2 {
		t.Errorf("Add(Red) again = %v (Len %d), want a duplicate at 1 and 3, not added", err, d.Len())
	}

	if _, err := d.Code(); message.For(err) != "That's 2 colors. A code needs exactly 5." {
		t.Errorf("Code() of 2 pegs: %q", message.For(err))
	}

	if err := d.Add(code.Color(99)); err == nil || d.Len() != 2 {
		t.Errorf("Add(invalid color) = %v (Len %d), want an error, not added", err, d.Len())
	}

	d.Add(code.Yellow)
	d.Add(code.Blue)
	d.Add(code.Black)
	err = d.Add(code.Pink)
	if !errors.Is(err, ErrDraftFull) || d.Len() != code.Length {
		t.Errorf("Add to a full row = %v (Len %d), want ErrDraftFull", err, d.Len())
	}
	if got, want := message.For(err), "The row is full. Submit it, or Undo to change a peg."; got != want {
		t.Errorf("full-row message = %q, want %q", got, want)
	}
	if got := err.Error(); got != "web: draft is full" {
		t.Errorf("full-row error text = %q", got)
	}
}

// TestDraftPegs checks how a draft is drawn: always 5 slots, the empty
// ones with no letter.
func TestDraftPegs(t *testing.T) {
	var d Draft
	d.Add(code.Pink)
	d.Add(code.White)
	want := []PegView{{"P", "Pink"}, {"W", "White"}, {}, {}, {}}
	got := d.Pegs()
	if len(got) != len(want) {
		t.Fatalf("Pegs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Pegs()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

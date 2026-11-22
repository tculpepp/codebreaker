package web

import (
	"slices"

	"github.com/tculpepp/codebreaker/internal/code"
)

// ErrDraftFull is returned by Draft.Add when all code.Length slots are
// already filled. It has a friendly message (see message.Friendly).
var ErrDraftFull error = draftFull{}

// draftFull is the type of ErrDraftFull.
type draftFull struct{}

// Error implements error.
func (draftFull) Error() string { return "web: draft is full" }

// Friendly returns the message shown to the player.
func (draftFull) Friendly() string {
	return "The row is full. Submit it, or Undo to change a peg."
}

// Draft is the row of pegs a player is building before they Submit it, as
// a guess or as a secret.
//
// It refuses a color that's already in the row, so a finished draft only
// ever fails code.Validate by being too short. The zero Draft is an empty
// row, ready to use.
type Draft struct {
	colors []code.Color
}

// Add puts c in the next empty slot. It returns ErrDraftFull when the row
// is full, a *code.DuplicateColorError when c is already in the row, and a
// *code.InvalidColorError for a color outside the palette. A refused color
// isn't added.
func (d *Draft) Add(c code.Color) error {
	next := len(d.colors) + 1 // the 1-based position c would take
	switch {
	case !c.Valid():
		return &code.InvalidColorError{Char: c.Abbrev(), Pos: next}
	case len(d.colors) == code.Length:
		return ErrDraftFull
	}
	// Go note: slices.Index returns the position of the first element
	// equal to c, or -1 if there's none.
	if i := slices.Index(d.colors, c); i >= 0 {
		return &code.DuplicateColorError{Color: c, First: i + 1, Second: next}
	}
	d.colors = append(d.colors, c)
	return nil
}

// Undo removes the last peg. It reports false, and does nothing, when the
// row is already empty.
func (d *Draft) Undo() bool {
	if len(d.colors) == 0 {
		return false
	}
	d.colors = d.colors[:len(d.colors)-1]
	return true
}

// Clear empties the row, ready for the next guess.
func (d *Draft) Clear() { d.colors = nil }

// Len returns how many pegs are in the row.
func (d *Draft) Len() int { return len(d.colors) }

// Code returns the finished row as a code. A row that isn't full returns
// a *code.LengthError, whose friendly message says how many pegs are
// missing.
func (d *Draft) Code() (code.Code, error) {
	return code.New(d.colors...)
}

// Pegs describes the row for the page: always code.Length slots, the
// empty ones as zero PegViews.
func (d *Draft) Pegs() []PegView {
	pegs := make([]PegView, code.Length)
	for i, c := range d.colors {
		pegs[i] = pegView(c)
	}
	return pegs
}

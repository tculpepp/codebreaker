package code

import (
	"fmt"
	"unicode"
)

// Color is one of the 8 peg colors (requirements.md, "Color Palette").
//
// Go note: `type Color uint8` makes a new named type whose underlying type is
// uint8 (a byte). It is a distinct type: you cannot pass a plain uint8 where a
// Color is expected without converting it with Color(x). That lets the
// compiler catch mix-ups, and it lets us attach methods (Name, Abbrev, ...).
// uint8 is plenty for 8 values and keeps a Code at just 5 bytes.
type Color uint8

// The 8 colors, in the order the requirements list them.
//
// Go note: inside a const block, `iota` starts at 0 and goes up by 1 on each
// line. Lines that leave out the type and value repeat the previous line's
// expression, so Black=0, White=1, ... Orange=7.
//
// Go note: NumColors is a common Go trick. Because it comes last, it equals
// the number of colors above it (8) and updates itself if a color is added.
// It is NOT a valid color, which Valid reports.
const (
	Black Color = iota
	White
	Red
	Blue
	Green
	Yellow
	Pink
	Orange
	NumColors // count of colors, not a color itself
)

// colorInfo describes how one color is shown and typed.
type colorInfo struct {
	abbrev rune   // the letter used when typing a guess
	name   string // the full English name
}

// colorTable is the single place that maps each Color to its letter and
// name. Every lookup in this file reads from it.
//
// Go note: `[NumColors]colorInfo` is an array, and its length is part of its
// type. The `Black: {...}` syntax (an "indexed composite literal") puts each
// entry at its constant's index. If NumColors changed and an entry were
// missing or out of range, the code would fail to compile (or the tests
// would catch the gap), instead of silently using wrong data.
var colorTable = [NumColors]colorInfo{
	Black:  {'K', "Black"}, // K, because B is taken by Blue
	White:  {'W', "White"},
	Red:    {'R', "Red"},
	Blue:   {'B', "Blue"},
	Green:  {'G', "Green"},
	Yellow: {'Y', "Yellow"},
	Pink:   {'P', "Pink"},
	Orange: {'O', "Orange"},
}

// Valid reports whether c is one of the 8 real colors. A Color made from an
// arbitrary number, such as Color(42), or NumColors itself, is not valid.
//
// Go note: `(c Color)` before the name makes this a method with a "value
// receiver": it gets its own copy of c. For a 1-byte type a copy is free,
// and a value receiver signals that the method doesn't change c.
func (c Color) Valid() bool {
	// Color is unsigned, so it can never be below 0. Only the top of the
	// range needs checking.
	return c < NumColors
}

// Abbrev returns the letter used to type c in a guess, such as 'R' for Red.
// It returns '?' for an invalid color, so bad data shows up clearly in
// output instead of crashing.
func (c Color) Abbrev() rune {
	if !c.Valid() {
		return '?'
	}
	return colorTable[c].abbrev
}

// Name returns the full English name of c, such as "Red". It returns a
// placeholder like "Color(42)" for an invalid color.
func (c Color) Name() string {
	if !c.Valid() {
		// Go note: uint8(c) converts to the plain number first. Inside a
		// String method, formatting c itself with %v would call String
		// again and recurse forever. Converting is the standard guard.
		return fmt.Sprintf("Color(%d)", uint8(c))
	}
	return colorTable[c].name
}

// String returns the same text as Name. Having it makes Color satisfy
// fmt.Stringer, so fmt.Println(Red) prints "Red" rather than "2".
//
// Go note: fmt.Stringer is the interface `interface{ String() string }`.
// Go interfaces are satisfied implicitly: Color never says "implements
// Stringer". Having a method with the right name and signature is enough,
// and the fmt package checks for it at run time when printing.
func (c Color) String() string {
	return c.Name()
}

// ColorFromRune looks up the color for a typed letter, ignoring case:
// 'r' and 'R' both give Red. The second result is false when the letter
// is not one of the 8 color letters.
//
// Go note: returning (value, ok bool) is Go's "comma ok" idiom, used for
// lookups that can miss (as with maps). It's lighter than an error when
// there's only one way to fail. The caller writes:
//
//	c, ok := ColorFromRune(r)
//	if !ok { ... }
func ColorFromRune(r rune) (Color, bool) {
	upper := unicode.ToUpper(r)

	// A loop over 8 entries is simple and fast enough. A map would add
	// setup code with no real speed benefit at this size.
	//
	// Go note: `for i, info := range colorTable` walks the array, giving
	// the index i and a copy of each element. i is an int, so it's
	// converted to Color for the return.
	for i, info := range colorTable {
		if info.abbrev == upper {
			return Color(i), true
		}
	}
	// Go note: on failure, return the zero value (0, which is Black) plus
	// false. Callers must check ok before using the color.
	return 0, false
}

// AllColors returns the 8 colors in palette order. Each call returns a new
// slice, so callers may change it without affecting anyone else.
//
// Go note: a slice ([]Color) is a small view onto an underlying array. If we
// handed out a shared slice, one caller could change the elements and every
// other caller would see it. Building a fresh slice avoids that.
func AllColors() []Color {
	// Go note: make([]Color, 0, NumColors) creates an empty slice with room
	// for 8 elements, so append never needs to grow it.
	colors := make([]Color, 0, NumColors)
	for c := Color(0); c < NumColors; c++ {
		colors = append(colors, c)
	}
	return colors
}

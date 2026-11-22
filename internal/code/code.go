package code

import "strings"

// Length is the number of pegs in a code (requirements.md: 5 positions).
const Length = 5

// Code is a sequence of Length colors: either the secret or a guess.
// A valid Code uses each color at most once (D7); use New, Parse or
// Validate to make sure.
//
// Go note: Code is an array ([5]Color), not a slice ([]Color). The length
// is part of an array's type, which gives three benefits:
//   - Its size can never be wrong at run time: there's no "4-color" Code.
//   - Arrays are comparable, so `a == b` works and a Code can be a map key.
//   - Assigning or passing a Code copies all 5 bytes. Nobody can change
//     your Code through a shared reference, which is how the game keeps
//     the secret safe from other packages.
//
// The cost: the zero value, `var c Code`, is five Blacks, which is invalid.
// That's why Validate exists.
type Code [Length]Color

// New builds a Code from exactly Length colors and checks it. Problems are
// reported in a fixed order: an out-of-range color first, then the wrong
// count, then a repeated color.
//
// Go note: `colors ...Color` is a variadic parameter. Callers can write
// New(Red, Green, Yellow, Blue, Black), and inside the function colors is a
// []Color. To pass an existing slice, write New(mySlice...).
func New(colors ...Color) (Code, error) {
	// Check every color first, matching the order Parse uses (letter,
	// then length, then duplicate).
	for i, c := range colors {
		if !c.Valid() {
			// Go note: when returning an error, the other results are by
			// convention zero values. `Code{}` is an empty Code literal.
			return Code{}, &InvalidColorError{Char: c.Abbrev(), Pos: i + 1}
		}
	}
	if len(colors) != Length {
		return Code{}, &LengthError{Got: len(colors)}
	}

	var c Code
	// Go note: the built-in copy(dst, src) copies elements from one slice to
	// another. c[:] makes a slice that views the whole array, so the colors
	// are copied straight into c.
	copy(c[:], colors)

	if err := c.Validate(); err != nil {
		return Code{}, err
	}
	return c, nil
}

// Validate reports whether c follows the rules: every color is in range
// and no color repeats. It returns nil when c is valid, an
// *InvalidColorError for an out-of-range color, or a *DuplicateColorError
// for the first repeated color found.
//
// "First" means the earliest second copy: for R G G B R it reports Green
// (positions 2 and 3), not Red (positions 1 and 5). A single fixed rule
// keeps error messages and tests predictable.
func (c Code) Validate() error {
	for i, color := range c {
		if !color.Valid() {
			return &InvalidColorError{Char: color.Abbrev(), Pos: i + 1}
		}
	}

	// For each position j, look back at every earlier position i. The
	// first match found is the earliest second copy. With only 5 pegs, the
	// 10 comparisons this takes cost nothing.
	for j := 1; j < Length; j++ {
		for i := 0; i < j; i++ {
			if c[i] == c[j] {
				return &DuplicateColorError{Color: c[j], First: i + 1, Second: j + 1}
			}
		}
	}
	return nil
}

// String returns the code as its 5 color letters, such as "RGYBK".
// That's also a format Parse accepts, so Parse(c.String()) gives back c.
func (c Code) String() string {
	// Go note: strings.Builder builds a string piece by piece without
	// creating a new string at every step (Go strings can't be changed, so
	// `s += x` makes a fresh copy each time). Grow reserves room up front.
	var b strings.Builder
	b.Grow(Length)
	for _, color := range c {
		b.WriteRune(color.Abbrev())
	}
	return b.String()
}

// mask returns a bit set with one bit per color in c: bit n is 1 when
// Color(n) appears. Score uses it to count shared colors quickly.
//
// Go note: there are exactly 8 colors, so one uint8 (8 bits) holds the
// whole set. `1 << color` shifts a 1 left by the color's number: Black
// (0) is 0b00000001, White (1) is 0b00000010, and so on. `|=` (bitwise OR)
// turns that bit on.
//
//	Code R G Y B K → colors 2 4 5 3 0 → mask 0b00111101
func (c Code) mask() uint8 {
	var m uint8
	for _, color := range c {
		m |= 1 << color
	}
	return m
}

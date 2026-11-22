package code

import "unicode"

// Parse reads a code typed by a player. It accepts the letters
// K W R B G Y P O in either case, run together or separated by spaces,
// tabs or commas: "RGYBK", "r g y b k" and "R,G,Y,B,K" all give the same
// code. Leading and trailing whitespace is ignored.
//
// Problems are reported in a fixed order, one per call
// (implementation-plan.md §7):
//  1. *InvalidColorError for the first character that isn't a color letter
//     or a separator (Pos counts pegs, not characters)
//  2. *LengthError if there aren't exactly Length colors
//  3. *DuplicateColorError if a color repeats
//
// On error, the returned Code is the zero value and must not be used.
func Parse(s string) (Code, error) {
	// Collect the colors into a slice first, because the input might hold
	// more than Length of them and we need the true count for LengthError.
	colors := make([]Color, 0, Length)

	// Go note: ranging over a string gives runes (Unicode characters), not
	// bytes. A multi-byte character such as 'é' comes through as one rune,
	// so positions stay right for any input. If the bytes aren't valid
	// UTF-8, range yields the replacement character U+FFFD, which is simply
	// reported as an invalid letter.
	for _, r := range s {
		if isSeparator(r) {
			continue
		}
		c, ok := ColorFromRune(r)
		if !ok {
			// len(colors)+1 is this peg's 1-based position. Separators
			// were skipped, so "R G X" reports X at position 3, not 5.
			return Code{}, &InvalidColorError{Char: r, Pos: len(colors) + 1}
		}
		colors = append(colors, c)
	}

	// Every character was a valid letter. New handles the remaining checks
	// (length, then duplicates), so the rules live in one place.
	return New(colors...)
}

// isSeparator reports whether r may appear between color letters:
// any whitespace (space, tab, newline) or a comma.
func isSeparator(r rune) bool {
	return unicode.IsSpace(r) || r == ','
}

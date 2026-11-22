package code

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// TestParseAccepted covers every input format the requirements accept:
// letters run together, separated by spaces or commas, either case, with
// extra whitespace around them.
func TestParseAccepted(t *testing.T) {
	want := Code{Red, Green, Yellow, Blue, Black}
	inputs := []string{
		"RGYBK",
		"rgybk",
		"RgYbK",
		"R G Y B K",
		"R,G,Y,B,K",
		"R, G, Y, B, K",
		"  RGYBK  ",
		"\tR G\tY B K\n", // tabs and a trailing newline (as read from the terminal)
		"RG YBK",         // separators in odd places are ignored
		"R,,G,Y,B,K",
	}
	for _, in := range inputs {
		// Go note: %q in a subtest name makes whitespace visible, so a
		// failing case reads "TestParseAccepted/\"\\tR_G...\"".
		t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
			got, err := Parse(in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", in, err)
			}
			if got != want {
				t.Errorf("Parse(%q) = %v, want %v", in, got, want)
			}
		})
	}
}

// TestParseErrors checks each error and its exact details. It also checks
// the reporting order from implementation-plan.md §7: invalid letter, then
// length, then duplicate. Inputs with more than one problem prove the order.
func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"empty", "", &LengthError{Got: 0}},
		{"only spaces", "   ", &LengthError{Got: 0}},
		{"too short", "RGYB", &LengthError{Got: 4}},
		{"too long", "RGYBKW", &LengthError{Got: 6}},
		{"all eight", "KWRBGYPO", &LengthError{Got: 8}},
		{"bad letter", "RGXBK", &InvalidColorError{Char: 'X', Pos: 3}},
		{"digit", "RG1BK", &InvalidColorError{Char: '1', Pos: 3}},
		// The position counts pegs, not characters: X is the 3rd peg even
		// though it's the 5th character.
		{"bad letter with spaces", "R G X B K", &InvalidColorError{Char: 'X', Pos: 3}},
		{"keeps typed case", "RGxBK", &InvalidColorError{Char: 'x', Pos: 3}},
		{"duplicate", "RGYRK", &DuplicateColorError{Color: Red, First: 1, Second: 4}},
		{"duplicate any case", "RGYrK", &DuplicateColorError{Color: Red, First: 1, Second: 4}},

		// Ordering: these have several problems; only the first in the
		// fixed order is reported.
		{"bad letter beats length", "RX", &InvalidColorError{Char: 'X', Pos: 2}},
		{"length beats duplicate", "RRG", &LengthError{Got: 3}},
		{"bad letter beats duplicate", "RRXBK", &InvalidColorError{Char: 'X', Pos: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Fatalf("Parse(%q) error = %#v, want %#v", tt.in, err, tt.wantErr)
			}
			// On error, the Code result must be the zero value, so a
			// careless caller can't use half-parsed data.
			if got != (Code{}) {
				t.Errorf("Parse(%q) returned %v alongside an error, want zero Code", tt.in, got)
			}
		})
	}
}

// FuzzParse feeds Parse random strings to look for crashes and broken
// promises. Run it for real with:
//
//	go test -fuzz=FuzzParse ./internal/code
//
// Without -fuzz, `go test` only runs the seed inputs added with f.Add, so
// it stays fast.
//
// Go note: fuzzing (built into Go since 1.18) generates inputs by mutating
// the seeds. When it finds a failing input, it saves it under
// testdata/fuzz/FuzzParse/ so that input is re-run by every later
// `go test`, turning each bug found into a permanent regression test.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"RGYBK", "r g y b k", "R,G,Y,B,K", "", "RRGYB", "XYZ", "KWRBGYPO", "ⓇGYBK"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		c, err := Parse(in) // must never panic, whatever the input

		if err != nil {
			// Every failure must be one of our three error types, so the
			// UI can always build a friendly message.
			var ic *InvalidColorError
			var le *LengthError
			var de *DuplicateColorError
			if !errors.As(err, &ic) && !errors.As(err, &le) && !errors.As(err, &de) {
				t.Fatalf("Parse(%q) returned unexpected error type %T: %v", in, err, err)
			}
			return
		}

		// Success promises: the code is valid, and printing then parsing
		// it again gives the same code (a "round trip").
		if verr := c.Validate(); verr != nil {
			t.Fatalf("Parse(%q) = %v, which fails Validate: %v", in, c, verr)
		}
		again, err := Parse(c.String())
		if err != nil || again != c {
			t.Fatalf("round trip of %v via %q gave (%v, %v)", c, c.String(), again, err)
		}
	})
}

// ExampleParse is a runnable example. `go test` runs it and compares what it
// prints with the "Output:" comment, and `go doc` shows it in the docs.
//
// Go note: example functions are named Example<Name>. They act as tests
// and as documentation that can't go out of date.
func ExampleParse() {
	c, err := Parse("r g y b k")
	fmt.Println(c, err)

	_, err = Parse("RGYRK")
	fmt.Println(err)
	// Output:
	// RGYBK <nil>
	// Red is used twice (positions 1 and 4)
}

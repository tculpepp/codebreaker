package code

import (
	"reflect"
	"testing"
)

// TestNewValid checks that New accepts 5 different valid colors and keeps
// them in the order given.
func TestNewValid(t *testing.T) {
	c, err := New(Red, Green, Yellow, Blue, Black)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	want := Code{Red, Green, Yellow, Blue, Black}
	// Go note: arrays are "comparable": == compares every element. This is
	// one reason Code is an array and not a slice; slices can't be compared
	// with == (only to nil).
	if c != want {
		t.Errorf("New = %v, want %v", c, want)
	}
}

// TestNewErrors checks each way New can fail, and that the error carries
// the exact details (positions are 1-based, as a player would count).
//
// Go note: reflect.DeepEqual compares values deeply. For two pointers it
// compares what they point to, so &LengthError{Got: 4} equals another
// &LengthError{Got: 4} even though they're different pointers. That lets one
// table check both the error's type and its fields.
func TestNewErrors(t *testing.T) {
	tests := []struct {
		name    string
		colors  []Color
		wantErr error
	}{
		{"none", nil, &LengthError{Got: 0}},
		{"too few", []Color{Red, Green, Yellow, Blue}, &LengthError{Got: 4}},
		{"too many", []Color{Red, Green, Yellow, Blue, Black, White}, &LengthError{Got: 6}},
		{"out-of-range color", []Color{Red, Green, Color(42), Blue, Black}, &InvalidColorError{Char: '?', Pos: 3}},
		{"duplicate", []Color{Red, Green, Yellow, Red, Black}, &DuplicateColorError{Color: Red, First: 1, Second: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Go note: `tt.colors...` spreads a slice into a variadic
			// parameter (New takes `colors ...Color`).
			_, err := New(tt.colors...)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("New error = %#v, want %#v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateZeroCode documents a Go detail: the zero value of Code is
// five Blacks (Black is 0), which breaks the "no duplicates" rule. Validate
// must catch it, so an uninitialized Code can't sneak through as a secret.
func TestValidateZeroCode(t *testing.T) {
	var c Code // Go note: every Go variable starts at its type's zero value; for an array of uint8s that's all zeros.
	want := &DuplicateColorError{Color: Black, First: 1, Second: 2}
	if err := c.Validate(); !reflect.DeepEqual(err, want) {
		t.Errorf("Validate() = %#v, want %#v", err, want)
	}
}

// TestValidateOutOfRange checks that Validate rejects a bad Color value even
// in a Code built directly as a literal, which bypasses New's checks.
func TestValidateOutOfRange(t *testing.T) {
	c := Code{Red, Green, Color(42), Blue, Black}
	want := &InvalidColorError{Char: '?', Pos: 3}
	if err := c.Validate(); !reflect.DeepEqual(err, want) {
		t.Errorf("Validate() = %#v, want %#v", err, want)
	}
}

// TestValidateReportsFirstDuplicate checks the rule for choosing which
// duplicate to report: the earliest second occurrence, paired with its
// first occurrence. One fixed rule keeps messages predictable.
func TestValidateReportsFirstDuplicate(t *testing.T) {
	// Green repeats at 2 and 3; Red repeats at 1 and 5. Green's second copy
	// (position 3) comes first, so Green is reported.
	c := Code{Red, Green, Green, Blue, Red}
	want := &DuplicateColorError{Color: Green, First: 2, Second: 3}
	if err := c.Validate(); !reflect.DeepEqual(err, want) {
		t.Errorf("Validate() = %#v, want %#v", err, want)
	}
}

// TestCodeString checks the compact letter form used in messages and tests.
func TestCodeString(t *testing.T) {
	c := Code{Red, Green, Yellow, Blue, Black}
	if got := c.String(); got != "RGYBK" {
		t.Errorf("String() = %q, want %q", got, "RGYBK")
	}
}

// TestCodeAsMapKey shows that a Code can be a map key, which the game uses
// to spot repeated guesses and the tests use to find duplicates.
//
// Go note: any comparable type can be a map key. Arrays of comparable
// elements qualify; slices do not.
func TestCodeAsMapKey(t *testing.T) {
	seen := map[Code]int{}
	seen[Code{Red, Green, Yellow, Blue, Black}] = 1
	if seen[Code{Red, Green, Yellow, Blue, Black}] != 1 {
		t.Error("an equal Code did not find the map entry")
	}
}

// TestCodeIsCopiedByValue shows array value semantics: assigning a Code
// copies all 5 colors, so changing the copy leaves the original alone.
func TestCodeIsCopiedByValue(t *testing.T) {
	original := Code{Red, Green, Yellow, Blue, Black}
	copied := original
	copied[0] = White
	if original[0] != Red {
		t.Error("changing the copy changed the original")
	}
}

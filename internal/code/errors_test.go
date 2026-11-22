package code

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrorMessages pins down the text of each error's Error() method.
// These are developer-facing messages (logs, test output). The friendly
// player-facing wording is produced separately by message.For.
func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"invalid letter", &InvalidColorError{Char: 'X', Pos: 3}, `position 3: 'X' is not a color letter`},
		{"too short", &LengthError{Got: 4}, "got 4 colors, want 5"},
		{"too long", &LengthError{Got: 7}, "got 7 colors, want 5"},
		{"duplicate", &DuplicateColorError{Color: Red, First: 1, Second: 4}, "Red is used twice (positions 1 and 4)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestErrorsAs shows, and checks, how callers get the details back out of
// an error that has been wrapped by other code.
//
// Go note: code higher up often wraps errors to add context with
// fmt.Errorf("...: %w", err). The %w verb keeps the original error inside.
// errors.As then searches that chain for an error of a given type and, if
// found, stores it in the target. The target is a pointer to a variable of
// the type you want; here that type is itself a pointer (*LengthError),
// because the methods are defined on the pointer type.
func TestErrorsAs(t *testing.T) {
	wrapped := fmt.Errorf("reading guess: %w", &LengthError{Got: 3})

	var lengthErr *LengthError
	if !errors.As(wrapped, &lengthErr) {
		t.Fatalf("errors.As did not find *LengthError in %v", wrapped)
	}
	if lengthErr.Got != 3 {
		t.Errorf("Got = %d, want 3", lengthErr.Got)
	}

	// A different error type must not match.
	var dupErr *DuplicateColorError
	if errors.As(wrapped, &dupErr) {
		t.Error("errors.As wrongly matched *DuplicateColorError")
	}
}

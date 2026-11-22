package message

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
)

// mustParse turns a code string into a code.Code, failing the test on a
// typo in the test itself.
func mustParse(t *testing.T, s string) code.Code {
	t.Helper()
	c, err := code.Parse(s)
	if err != nil {
		t.Fatalf("bad code %q in test: %v", s, err)
	}
	return c
}

// selfExplaining is an error that carries its own message for the player,
// as player.ErrSecretMismatch and the terminal's menu errors do.
type selfExplaining struct{}

// Error implements error.
func (selfExplaining) Error() string { return "player: secrets differ" }

// Friendly implements Friendly.
func (selfExplaining) Friendly() string { return "The two codes don't match." }

// TestFor checks the message shown for each kind of mistake. The real
// errors come from code.Parse and game.CheckRepeat, so this also proves the
// error types are matched correctly, even when wrapped. These are the same
// words the terminal has always shown; the terminal's golden files check
// that too.
func TestFor(t *testing.T) {
	parseErr := func(s string) error {
		_, err := code.Parse(s)
		return err
	}
	repeat := &game.RepeatGuessError{Guess: mustParse(t, "RGYBK"), Row: 2}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "invalid letter", err: parseErr("RGXBK"),
			want: "'X' isn't a color letter (position 3). Use K W R B G Y P O."},
		{name: "nothing typed", err: parseErr(""),
			want: "Type 5 color letters, like RGYBK."},
		{name: "one color", err: parseErr("R"),
			want: "That's 1 color. A code needs exactly 5."},
		{name: "too few", err: parseErr("RGYB"),
			want: "That's 4 colors. A code needs exactly 5."},
		{name: "too many", err: parseErr("RGYBKW"),
			want: "That's 6 colors. A code needs exactly 5."},
		{name: "duplicate", err: parseErr("RGYRK"),
			want: "Red is used twice (positions 1 and 4). Each color may appear only once."},
		{name: "repeat guess", err: repeat,
			want: "You already tried RGYBK in row 2. Try a different code."},
		{name: "wrapped repeat", err: fmt.Errorf("checking: %w", repeat),
			want: "You already tried RGYBK in row 2. Try a different code."},
		{name: "friendly", err: selfExplaining{}, want: "The two codes don't match."},
		{name: "wrapped friendly", err: fmt.Errorf("making secret: %w", selfExplaining{}),
			want: "The two codes don't match."},
		{name: "anything else", err: errors.New("disk on fire"), want: "disk on fire"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := For(tt.err); got != tt.want {
				t.Errorf("For = %q\nwant  %q", got, tt.want)
			}
		})
	}
}

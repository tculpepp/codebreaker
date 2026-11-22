package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
)

// TestChoiceErrorIsFriendly checks the message shown for a bad menu
// answer. The other mistakes' messages are tested in package message.
func TestChoiceErrorIsFriendly(t *testing.T) {
	r, out := newTestRenderer()
	r.Error(fmt.Errorf("menu: %w", &ChoiceError{Input: "7", N: 3}))
	if want := " ! Please type a number from 1 to 3.\n"; out.String() != want {
		t.Errorf("Error wrote %q, want %q", out.String(), want)
	}
}

// TestErrorWritesFriendlyMessage checks that Error shows the friendly
// text, marked so it stands out above the repeated prompt.
func TestErrorWritesFriendlyMessage(t *testing.T) {
	r, out := newTestRenderer()
	_, err := code.Parse("RGYB")
	r.Error(err)
	if want := " ! That's 4 colors. A code needs exactly 5.\n"; out.String() != want {
		t.Errorf("Error wrote %q, want %q", out.String(), want)
	}
}

// TestParseChoice checks menu answers: a number in range gives its
// 0-based index; anything else is a *ChoiceError carrying what was typed.
func TestParseChoice(t *testing.T) {
	tests := []struct {
		typed string
		want  int
		ok    bool
	}{
		{"1", 0, true},
		{"3", 2, true},
		{" 2 ", 1, true},
		{"0", 0, false},
		{"4", 0, false},
		{"-1", 0, false},
		{"two", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseChoice(tt.typed, 3)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Errorf("ParseChoice(%q, 3) = %d, %v; want %d, nil", tt.typed, got, err, tt.want)
			}
			continue
		}
		var ce *ChoiceError
		if !errors.As(err, &ce) || ce.Input != tt.typed || ce.N != 3 {
			t.Errorf("ParseChoice(%q, 3) error = %v, want *ChoiceError{%q, 3}", tt.typed, err, tt.typed)
		}
	}
}

// TestChoiceErrorMessage checks the plain error text, used in logs rather
// than on screen.
func TestChoiceErrorMessage(t *testing.T) {
	err := &ChoiceError{Input: "x", N: 2}
	if got, want := err.Error(), `ui: choice "x" is not a number from 1 to 2`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestScreens checks the fixed screens: the welcome banner, the help text
// and a menu.
func TestScreens(t *testing.T) {
	tests := []struct {
		name string
		draw func(r *Renderer)
	}{
		{name: "welcome", draw: func(r *Renderer) { r.Welcome() }},
		{name: "help", draw: func(r *Renderer) { r.Help() }},
		{name: "menu", draw: func(r *Renderer) { r.Menu("Play again?", []string{"New game", "Quit"}) }},
		{name: "goodbye", draw: func(r *Renderer) { r.Goodbye() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out := newTestRenderer()
			tt.draw(r)
			checkGolden(t, tt.name, out.String())
		})
	}
}

// TestHelpScoringMatchesPoints checks that every bracket in the help text
// matches game.Points, so the help can't drift from the real rules.
func TestHelpScoringMatchesPoints(t *testing.T) {
	r, out := newTestRenderer()
	r.Help()
	for _, want := range []string{"1–4: 12", "5–6: 10", "7–8: 8", "9–10: 6", "11–12: 4"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help is missing %q:\n%s", want, out.String())
		}
	}
}

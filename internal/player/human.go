package player

import (
	"context"
	"errors"
	"fmt"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
	"codebreaker/internal/input"
)

// Screen is what a Human needs from the display: a way to report a mistake
// and a way to show the help text. *ui.Renderer satisfies it.
//
// Go note: Human could take a *ui.Renderer directly. Asking for this small
// interface instead says exactly what Human uses (two methods, not the
// whole renderer), and lets tests pass a fake that records the calls.
// This is the "accept interfaces" half of the Go saying "accept
// interfaces, return structs".
type Screen interface {
	Error(err error)
	Help()
}

// Human is a player at the keyboard.
//
// It only asks questions and reports mistakes. The board is drawn by the
// screen's Observer methods as the match plays, so Human never draws it.
//
// Go note: Human is built by combining two smaller parts, a Prompter and a
// Screen, held in its fields. Go has no inheritance; this kind of
// composition is how bigger behavior is built from small pieces.
type Human struct {
	in     input.Prompter
	screen Screen
}

// Go note: compile-time check that *Human can sit in a game.Seat. Human
// never mentions game.Participant; having MakeSecret and Guess with the
// right signatures is all it takes.
var _ game.Participant = (*Human)(nil)

// NewHuman returns a player who answers through in and sees mistakes and
// help on screen.
func NewHuman(in input.Prompter, screen Screen) *Human {
	return &Human{in: in, screen: screen}
}

// Guess asks for the next guess until the player types a valid code that
// hasn't been tried this round. Mistakes are shown on the screen and
// don't use a row (D14).
//
// It returns input.ErrQuit, io.EOF or a context error unchanged when the
// player quits, the input ends, or Ctrl-C is pressed.
func (h *Human) Guess(ctx context.Context, info game.RoundInfo, history []game.Turn) (code.Code, error) {
	prompt := fmt.Sprintf(" Row %d/%d (? help, q quit) ▸ ", len(history)+1, info.MaxRows)

	// Go note: this is a "function literal" (an anonymous function). It
	// uses history from the surrounding function; a function that captures
	// variables like this is called a closure. ask calls it to run the
	// extra repeat check that only guesses need.
	notRepeat := func(c code.Code) error { return game.CheckRepeat(history, c) }
	return h.ask(ctx, prompt, notRepeat)
}

// MakeSecret asks the player for a secret code until they type a valid
// one.
//
// The code is typed in plain sight. That's fine against the computer,
// which never looks; hidden typing for Human vs Human arrives in v0.5.0.
func (h *Human) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	prompt := fmt.Sprintf(" %s, choose a secret code for %s ▸ ", info.Maker.Name, info.Breaker.Name)
	return h.ask(ctx, prompt, nil)
}

// ask shows prompt and reads lines until one parses as a code and passes
// check (if check isn't nil). Help requests show the help and ask again;
// other errors from the prompter end the loop and are returned.
func (h *Human) ask(ctx context.Context, prompt string, check func(code.Code) error) (code.Code, error) {
	// Go note: `for { ... }` with no condition loops forever. The loop
	// ends with a return, and `continue` jumps straight to the next round
	// of the loop.
	for {
		line, err := h.in.Line(ctx, prompt)
		if errors.Is(err, input.ErrHelp) {
			h.screen.Help()
			continue
		}
		if err != nil {
			return code.Code{}, err
		}

		// Errors are checked in a fixed order: Parse finds letter, length
		// and duplicate problems (in that order), then check finds a
		// repeat. Only the first problem is reported each time.
		c, err := code.Parse(line)
		if err == nil && check != nil {
			err = check(c)
		}
		if err != nil {
			h.screen.Error(err)
			continue
		}
		return c, nil
	}
}

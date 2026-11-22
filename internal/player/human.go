package player

import (
	"context"
	"errors"
	"fmt"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/input"
)

// Screen is what a Human needs from the display: a way to report a
// mistake, show the help text, hand the keyboard to the other player, draw
// the board again afterwards, and format prompts in the screen's style
// (Unicode or ASCII). *ui.Renderer satisfies it.
//
// Go note: Human could take a *ui.Renderer directly. Asking for this small
// interface instead says exactly what Human uses (five methods, not the
// whole renderer), and lets tests pass a fake that records the calls.
// This is the "accept interfaces" half of the Go saying "accept
// interfaces, return structs".
type Screen interface {
	Error(err error)
	Help()
	Handoff(next string)
	Redraw()
	Prompt(text string) string
}

// ErrSecretMismatch is reported (to the Screen, not returned) when the two
// hidden entries of a secret code differ. The Maker then types it again.
var ErrSecretMismatch error = secretMismatch{}

// secretMismatch is the type behind ErrSecretMismatch.
//
// Go note: it's an empty struct, which takes no memory, because the error
// needs no data, only methods. Besides Error it has a Friendly method,
// which satisfies message.Friendly without importing message, so the
// screen shows a friendly message for it.
type secretMismatch struct{}

// Error implements error.
func (secretMismatch) Error() string { return "player: the two secret codes differ" }

// Friendly returns the message shown to the player.
func (secretMismatch) Friendly() string {
	return "The two codes don't match. Type the secret again."
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

	// Hidden makes MakeSecret type the code hidden, twice, and then hand
	// the keyboard to the Breaker (D10). Set it when both players are
	// human. Against the computer there's no one to hide the code from.
	Hidden bool
}

// Go note: compile-time check that *Human can sit in a game.Seat. Human
// never mentions game.Participant; having MakeSecret and Guess with the
// right signatures is all it takes.
var _ game.Participant = (*Human)(nil)

// NewHuman returns a player who answers through in and sees mistakes and
// help on screen. It types secrets in plain sight; set Hidden for Human vs
// Human.
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
	prompt := h.screen.Prompt(fmt.Sprintf("Row %d/%d (? help, q quit)", len(history)+1, info.MaxRows))

	// Go note: this is a "function literal" (an anonymous function). It
	// uses history from the surrounding function; a function that captures
	// variables like this is called a closure. ask calls it to run the
	// extra repeat check that only guesses need.
	notRepeat := func(c code.Code) error { return game.CheckRepeat(history, c) }
	return h.ask(ctx, prompt, h.in.Line, notRepeat)
}

// MakeSecret asks the player for a secret code until they type a valid
// one.
//
// Normally the code is typed in plain sight, which is fine against the
// computer: it never looks. With Hidden set, the code is typed hidden and
// then again to check it, and the keyboard is handed to the Breaker, who
// presses Enter and sees the board again.
func (h *Human) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	maker, breaker := info.Maker.Name, info.Breaker.Name
	if !h.Hidden {
		prompt := h.screen.Prompt(fmt.Sprintf("%s, choose a secret code for %s", maker, breaker))
		return h.ask(ctx, prompt, h.in.Line, nil)
	}

	// Go note: h.in.Secret, without parentheses, is a "method value": the
	// Secret method bound to h.in, ready to be called later. Passing it to
	// ask (and h.in.Line elsewhere) lets one loop serve both kinds of
	// prompt.
	first := h.screen.Prompt(fmt.Sprintf("%s, type a secret code for %s (hidden)", maker, breaker))
	var secret code.Code
	for {
		c, err := h.ask(ctx, first, h.in.Secret, nil)
		if err != nil {
			return code.Code{}, err
		}
		again, err := h.ask(ctx, h.screen.Prompt("Type it again to check"), h.in.Secret, nil)
		if err != nil {
			return code.Code{}, err
		}
		if again == c {
			secret = c
			break
		}
		h.screen.Error(ErrSecretMismatch)
	}

	h.screen.Handoff(breaker)
	if err := h.waitForEnter(ctx, h.screen.Prompt(breaker+", press Enter when you're ready")); err != nil {
		return code.Code{}, err
	}
	h.screen.Redraw()
	return secret, nil
}

// waitForEnter shows prompt and waits for any answer. Help shows the help
// and asks again; other errors are returned.
func (h *Human) waitForEnter(ctx context.Context, prompt string) error {
	for {
		_, err := h.in.Line(ctx, prompt)
		if errors.Is(err, input.ErrHelp) {
			h.screen.Help()
			continue
		}
		return err
	}
}

// ask shows prompt and reads answers with read (the prompter's Line or
// Secret) until one parses as a code and passes check (if check isn't
// nil). Help requests show the help and ask again; other errors from the
// prompter end the loop and are returned.
func (h *Human) ask(ctx context.Context, prompt string,
	read func(context.Context, string) (string, error), check func(code.Code) error) (code.Code, error) {
	// Go note: `for { ... }` with no condition loops forever. The loop
	// ends with a return, and `continue` jumps straight to the next round
	// of the loop.
	for {
		line, err := read(ctx, prompt)
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

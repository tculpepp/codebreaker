package web

import (
	"context"
	"fmt"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
	"codebreaker/internal/message"
)

// Tapper is a human player on the web page. Its guesses and secrets are
// built from the player's taps (or key presses), one peg at a time.
//
// It's the web version of the terminal's player.Human. Like Human, it
// checks every mistake itself, so a mistake never costs a row.
type Tapper struct {
	s *Session
}

// Go note: a compile-time check that *Tapper can play both roles.
var _ game.Participant = (*Tapper)(nil)

// Guess shows the board with an empty draft and builds a guess from the
// player's taps until they Submit a valid one that isn't a repeat. Quit
// returns ErrQuit.
func (t *Tapper) Guess(ctx context.Context, info game.RoundInfo, history []game.Turn) (code.Code, error) {
	t.s.ownSecret = nil // the player's own code belongs to the round they made it for
	prompt := fmt.Sprintf("Row %d of %d", len(history)+1, info.MaxRows)
	return t.build(ctx, func(c code.Code) error { return game.CheckRepeat(history, c) },
		func(d *Draft, msg string) View {
			return View{
				Screen:  ScreenBoard,
				Lines:   []string{prompt},
				Board:   t.s.board.View(d),
				Message: msg,
				Input:   InputView{Palette: true, Undo: true, Submit: true},
			}
		})
}

// MakeSecret shows the secret screen and builds a code from the player's
// taps until they Submit a valid one. Quit returns ErrQuit.
func (t *Tapper) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	c, err := t.build(ctx, func(code.Code) error { return nil },
		func(d *Draft, msg string) View {
			return View{
				Screen:  ScreenSecret,
				Title:   "Make a secret code",
				Lines:   []string{info.Breaker.Name + " will try to break it."},
				Board:   &BoardView{Draft: d.Pegs(), MaxRows: info.MaxRows},
				Message: msg,
				Input:   InputView{Palette: true, Undo: true, Submit: true},
			}
		})
	if err == nil {
		// Remembered so the board can show the player their own code
		// while the computer tries to break it.
		t.s.ownSecret = &c
	}
	return c, err
}

// build runs the draft: it shows view after every event, with a message
// for any mistake, until Submit gives a full code that check accepts.
//
// Go note: Guess and MakeSecret differ only in their extra check and their
// screen, so each passes those in as functions. Functions are values in
// Go, and passing them in like this avoids writing the loop twice.
func (t *Tapper) build(ctx context.Context, check func(code.Code) error,
	view func(d *Draft, msg string) View) (code.Code, error) {
	var d Draft
	t.s.show(view(&d, ""))
	for {
		ev, err := t.s.next(ctx)
		if err != nil {
			return code.Code{}, err
		}
		var mistake error
		switch ev.Kind {
		case KindColor:
			mistake = d.Add(ev.Color())
		case KindUndo:
			d.Undo()
		case KindSubmit:
			c, err := d.Code()
			if err == nil {
				err = check(c)
			}
			if err == nil {
				return c, nil
			}
			mistake = err
		case KindQuit:
			return code.Code{}, ErrQuit
		default:
			continue // not for this screen: show nothing new
		}
		t.s.show(view(&d, message.For(mistake)))
	}
}

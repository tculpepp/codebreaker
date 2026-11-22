package web

import (
	"context"
	"fmt"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/player"
)

// pacedComputer is the computer player as the web page shows it. It makes
// and breaks codes exactly as player.Computer does, but before each guess
// it shows the board and pauses, so the player can watch the rows appear
// one at a time (Web App Requirement 5).
//
// Go note: pacedComputer wraps a *player.Computer and adds behavior
// before passing each call on. This is the "decorator" idea: same
// interface (game.Participant), a little extra work around it.
type pacedComputer struct {
	s *Session
	c *player.Computer
}

// Go note: a compile-time check that *pacedComputer can play both roles.
var _ game.Participant = (*pacedComputer)(nil)

// MakeSecret picks a random code, as player.Computer does. There's
// nothing to show: the player sees the board when they start guessing.
func (p *pacedComputer) MakeSecret(ctx context.Context, info game.RoundInfo) (code.Code, error) {
	return p.c.MakeSecret(ctx, info)
}

// Guess shows the board, with the player's own code and a Skip button,
// pauses, then guesses as player.Computer does.
func (p *pacedComputer) Guess(ctx context.Context, info game.RoundInfo, history []game.Turn) (code.Code, error) {
	s := p.s
	if len(history) == 0 {
		s.skipping = false // a new round is watched from the start
	}
	board := s.board.View(nil)
	if s.ownSecret != nil && board.Secret == nil {
		board.Secret = pegViews(*s.ownSecret)
	}
	s.show(View{
		Screen: ScreenBoard,
		Lines:  []string{fmt.Sprintf("%s is guessing: row %d of %d", info.Breaker.Name, len(history)+1, info.MaxRows)},
		Board:  board,
		Input:  InputView{Skip: !s.skipping},
	})
	if err := s.pause(ctx); err != nil {
		return code.Code{}, err
	}
	return p.c.Guess(ctx, info, history)
}

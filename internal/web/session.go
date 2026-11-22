package web

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"unicode/utf8"

	"codebreaker/internal/game"
	"codebreaker/internal/player"
)

// ErrQuit is returned by a Tapper when the player taps Quit. The session
// catches it, shows the standings and offers another game (D16).
var ErrQuit = errors.New("web: player quit")

// computerName is the computer player's name on screen.
const computerName = "Computer"

// maxName is the longest name kept, in characters, so names fit the board.
const maxName = 20

// Session is one visit to the web app: the screens, from the name to the
// end of each game, driven by the player's events. It's the web version of
// the terminal's app package.
//
// The browser calls Handle for each tap or key press, from its own
// callback, and Run plays the game on another goroutine. Handle never
// waits, so the page never freezes.
//
// Go note: two goroutines share the event queue, so a sync.Mutex guards
// it: only the goroutine holding the lock may touch queue. Everything else
// in Session is used only by Run's goroutine and needs no lock.
type Session struct {
	rng  *rand.Rand
	out  func(View) // draws a View on the page
	name string     // the player's name

	mu     sync.Mutex    // guards queue
	queue  []Event       // events not yet read by Run, oldest first
	notify chan struct{} // gets a value when queue may have grown

	current  View   // the latest View, shown again when help closes
	helpOpen bool   // help is showing over current
	board    *Board // the current game's board
}

// NewSession returns a session that uses rng for every random choice and
// show to draw each View. Call Run to start it.
func NewSession(rng *rand.Rand, show func(View)) *Session {
	return &Session{
		rng: rng,
		out: show,
		// Go note: a channel with room for one value works as a "doorbell".
		// Ringing it when it already holds a value does nothing (see
		// Handle), but one ring is enough to wake the reader, who then
		// takes everything in the queue.
		notify: make(chan struct{}, 1),
	}
}

// Handle queues one event from the page. It never blocks, whatever Run is
// doing, because it's called from a browser callback.
func (s *Session) Handle(ev Event) {
	s.mu.Lock()
	s.queue = append(s.queue, ev)
	s.mu.Unlock()
	// Go note: a select with a default case never waits. If the doorbell
	// already holds a value, the send can't happen and default runs
	// instead; the reader will still see this event when it wakes.
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Run shows the screens until ctx is cancelled, and then returns ctx's
// error. It never returns nil, because there's always another game to
// play.
func (s *Session) Run(ctx context.Context) error {
	s.show(View{
		Screen: ScreenName,
		Title:  "Codebreaker",
		Lines: []string{
			"Break the secret code: 5 different colors, chosen from 8.",
			"What's your name?",
		},
		Input: InputView{Text: true},
	})
	for {
		ev, err := s.next(ctx)
		if err != nil {
			return err
		}
		if ev.Kind == KindText {
			s.name = cleanName(ev.Text)
			break
		}
	}
	for {
		if err := s.playSingle(ctx); err != nil {
			return err
		}
	}
}

// playSingle shows the line-up, plays one Single game in which the player
// breaks the computer's code, and shows the result until Play again.
func (s *Session) playSingle(ctx context.Context) error {
	s.show(View{
		Screen: ScreenMenu,
		Title:  "Single game",
		Lines:  []string{"Blue " + s.name + " vs Red " + computerName + ".", computerName + " makes the code."},
		Menu:   &MenuView{Choices: []string{"Start"}},
	})
	if _, err := s.choose(ctx, 1); err != nil {
		return err
	}

	s.board = NewBoard()
	seats := [2]game.Seat{
		{Name: s.name, Peg: game.PegBlue, Player: &Tapper{s: s}},
		{Name: computerName, Peg: game.PegRed, Player: player.NewComputer(s.rng)},
	}
	// The computer makes the code (seat 1), so the player breaks it.
	// NewMatch only fails on a bad setup, which these fixed seats can't
	// be, so an error there would be a bug. It's still passed up.
	m, err := game.NewMatch(game.Config{Mode: game.ModeSingle, FirstMaker: game.FirstMakerSeat1}, seats, s.rng, s.board)
	if err == nil {
		_, err = m.Play(ctx)
	}
	if err != nil && !errors.Is(err, ErrQuit) {
		return err
	}

	s.show(View{
		Screen: ScreenResult,
		Title:  "Game over",
		Lines:  s.summary(),
		Board:  s.board.View(nil),
		Menu:   &MenuView{Choices: []string{"Play again"}},
	})
	_, err = s.choose(ctx, 1)
	return err
}

// summary describes how the game ended, in the terminal's words.
func (s *Session) summary() []string {
	res, ok := s.board.Result()
	switch {
	case !ok:
		return []string{"Game stopped before the code was cracked."}
	case !res.Solved:
		return []string{fmt.Sprintf("%s didn't crack the code in %d rows.", s.name, res.Rows),
			"No points this round."}
	default:
		lo, hi := game.Bracket(res.Rows)
		return []string{
			fmt.Sprintf("%s cracked the code in %d %s!", s.name, res.Rows, plural(res.Rows, "row")),
			fmt.Sprintf("In a match that would score %d points (solved in rows %d–%d).", res.Points, lo, hi),
		}
	}
}

// choose waits for a menu choice from 0 to n-1 and returns it. Other
// events are ignored.
func (s *Session) choose(ctx context.Context, n int) (int, error) {
	for {
		ev, err := s.next(ctx)
		if err != nil {
			return 0, err
		}
		if ev.Kind == KindChoose && ev.Choice < n {
			return ev.Choice, nil
		}
	}
}

// show draws v and remembers it. While help is open, v is only
// remembered, and appears when help closes.
func (s *Session) show(v View) {
	s.current = v
	if !s.helpOpen {
		s.out(v)
	}
}

// next returns the next event for the current screen, waiting until there
// is one or ctx is cancelled. It handles help itself: Help shows the help
// sheet over the current screen, Close shows that screen again, and other
// events while help is open are dropped, since they were taps behind the
// sheet.
func (s *Session) next(ctx context.Context) (Event, error) {
	for {
		ev, err := s.pop(ctx)
		if err != nil {
			return Event{}, err
		}
		switch {
		case ev.Kind == KindHelp && !s.helpOpen:
			s.helpOpen = true
			s.out(helpView())
		case ev.Kind == KindClose && s.helpOpen:
			s.helpOpen = false
			s.out(s.current)
		case s.helpOpen, ev.Kind == KindHelp, ev.Kind == KindClose:
			// behind the help sheet, or help/close with nothing to do
		default:
			return ev, nil
		}
	}
}

// pop takes the oldest event from the queue, waiting for one if it's
// empty.
func (s *Session) pop(ctx context.Context) (Event, error) {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			ev := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return ev, nil
		}
		s.mu.Unlock()

		// Go note: select waits for whichever happens first: the doorbell
		// (a new event) or the context being cancelled.
		select {
		case <-s.notify:
		case <-ctx.Done():
			return Event{}, ctx.Err()
		}
	}
}

// cleanName trims spaces, uses "Player" for an empty name, and cuts a long
// one to maxName characters.
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Player"
	}
	if utf8.RuneCountInString(s) > maxName {
		// Go note: converting a string to []rune splits it into
		// characters, so the cut never lands in the middle of one, as
		// slicing the bytes (s[:20]) could with "ë".
		s = strings.TrimSpace(string([]rune(s)[:maxName]))
	}
	return s
}

// plural returns word, with an "s" added unless n is 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

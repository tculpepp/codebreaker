package web

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/player"
)

// ErrQuit is returned by a player when the person taps Quit. The session
// catches it, shows the standings and offers another game (D16).
var ErrQuit = errors.New("web: player quit")

// computerName is the computer player's name on screen.
const computerName = "Computer"

// maxName is the longest name kept, in characters, so names fit the board.
const maxName = 20

// Pacer makes the pause between the computer's guesses, so the player can
// follow them. Each call starts a pause and returns a channel that's
// closed when it's over.
//
// Go note: Pacer is a function type. Any function with this signature is a
// Pacer, so tests pass tiny functions that never sleep, and the browser
// passes Delay(700 * time.Millisecond). A one-method interface would work
// too; a function type is simpler when there's only one thing to do.
type Pacer func() <-chan struct{}

// Delay returns a Pacer that pauses for d.
func Delay(d time.Duration) Pacer {
	return func() <-chan struct{} {
		done := make(chan struct{})
		// Go note: time.AfterFunc runs the function on its own goroutine
		// after d. Closing a channel wakes everyone waiting on it.
		time.AfterFunc(d, func() { close(done) })
		return done
	}
}

// Session is one visit to the web app: the menus, and every game, driven
// by the player's events. It's the web version of the terminal's app
// package.
//
// The browser calls Handle for each tap or key press, from its own
// callback, and Run plays the game on another goroutine. Handle never
// waits, so the page never freezes.
//
// Go note: two goroutines share the event queue, so a sync.Mutex guards
// it: only the goroutine holding the lock may touch queue. Everything else
// in Session is used only by Run's goroutine and needs no lock.
type Session struct {
	rng     *rand.Rand
	pace    Pacer
	out     func(View) // draws a View on the page
	name    string     // the first player's name (seat 0, Blue)
	name2   string     // the second person's name (seat 1, Red), in Human vs Human
	vsHuman bool       // seat 1 is a person rather than the computer

	mu     sync.Mutex    // guards queue
	queue  []Event       // events not yet read by Run, oldest first
	notify chan struct{} // gets a value when queue may have grown

	current  View   // the latest View, shown again when help closes
	helpOpen bool   // help is showing over current
	board    *Board // the current game's board

	// The game chosen in the menus. "Play again" reuses it.
	mode  game.Mode
	easy  bool
	first game.FirstMaker

	ownSecret *code.Code // the player's own code, shown while the computer breaks it
	skipping  bool       // Skip was tapped: the computer's other guesses this round come at once
}

// NewSession returns a session that uses rng for every random choice, pace
// between the computer's guesses, and show to draw each View. Call Run to
// start it.
func NewSession(rng *rand.Rand, pace Pacer, show func(View)) *Session {
	return &Session{
		rng:  rng,
		pace: pace,
		out:  show,
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

// step is one screen of the menu flow. It does that screen's work and
// returns the next step.
//
// Go note: this is the same state machine idea as the terminal's app
// package, written with functions instead of numbered states: each step
// returns the function to run next. A type can refer to itself, so a step
// returns a step.
type step func(ctx context.Context) (step, error)

// Run shows the screens until ctx is cancelled, and then returns ctx's
// error. It never returns nil, because there's always another game to
// play.
func (s *Session) Run(ctx context.Context) error {
	next, err := s.askName(ctx)
	for err == nil {
		next, err = next(ctx)
	}
	return err
}

// askName asks for the first player's name, then goes to the main menu.
func (s *Session) askName(ctx context.Context) (step, error) {
	name, err := s.readName(ctx, View{
		Screen: ScreenName,
		Title:  "Codebreaker",
		Lines: []string{
			fmt.Sprintf("Break the secret code: %d different colors, chosen from %d.", code.Length, code.NumColors),
			"What's your name?",
		},
		Input: InputView{Text: true},
	}, "Player", "")
	s.name = name
	return s.mainMenu, err
}

// askSecondName asks for the second person's name, which must differ
// from the first, so the screens can tell them apart.
func (s *Session) askSecondName(ctx context.Context) (step, error) {
	name, err := s.readName(ctx, View{
		Screen: ScreenName,
		Title:  "Second player",
		Lines:  []string{"What's the second player's name?"},
		Input:  InputView{Text: true},
	}, "Player 2", s.name)
	s.name2 = name
	if s.mode == game.ModeSingle {
		return s.chooseRole, err
	}
	return s.play, err
}

// readName shows v and waits for a name: trimmed, cut to fit, def if
// empty, and refused with a message if it matches taken (ignoring case).
func (s *Session) readName(ctx context.Context, v View, def, taken string) (string, error) {
	s.show(v)
	for {
		ev, err := s.next(ctx)
		if err != nil {
			return "", err
		}
		if ev.Kind != KindText {
			continue
		}
		name := cleanName(ev.Text, def)
		// Go note: strings.EqualFold compares two strings ignoring case,
		// so "Tom" and "TOM" count as the same name.
		if strings.EqualFold(name, taken) {
			v.Message = taken + " is already taken. Please choose another name."
			s.show(v)
			continue
		}
		return name, nil
	}
}

// mainMenu offers a match or a Single game.
func (s *Session) mainMenu(ctx context.Context) (step, error) {
	choice, err := s.menu(ctx, "Main menu", nil,
		fmt.Sprintf("Match (first to %d points)", game.TargetScore),
		"Single game (one round)")
	if err != nil {
		return nil, err
	}
	s.mode = []game.Mode{game.ModeMatch, game.ModeSingle}[choice]
	// In a match, the coin always picks the first Maker (D6).
	s.first = game.FirstMakerCoinFlip
	return s.chooseLevel, nil
}

// chooseLevel asks for the difficulty (D18).
func (s *Session) chooseLevel(ctx context.Context) (step, error) {
	choice, err := s.menu(ctx, "Difficulty", nil,
		"Normal",
		"Easy (each result symbol shows which peg it's for)")
	if err != nil {
		return nil, err
	}
	s.easy = choice == 1
	return s.chooseOpponent, nil
}

// chooseOpponent asks whether to play the computer or another person at
// the same device. Computer vs Computer isn't offered (D17).
func (s *Session) chooseOpponent(ctx context.Context) (step, error) {
	choice, err := s.menu(ctx, "Play against:", nil, "The computer", "Another person")
	if err != nil {
		return nil, err
	}
	s.vsHuman = choice == 1
	switch {
	case s.vsHuman:
		return s.askSecondName, nil
	case s.mode == game.ModeSingle:
		return s.chooseRole, nil
	default:
		return s.play, nil
	}
}

// chooseRole asks who makes the code in a Single game (D13).
func (s *Session) chooseRole(ctx context.Context) (step, error) {
	choices := []string{
		"Break the computer's code",
		"Make a code for the computer to break",
		"Flip a coin",
	}
	if s.vsHuman {
		choices[0] = s.name + " breaks " + s.name2 + "'s code"
		choices[1] = s.name2 + " breaks " + s.name + "'s code"
	}
	choice, err := s.menu(ctx, "Single game: who makes the code?", nil, choices...)
	if err != nil {
		return nil, err
	}
	// The first player sits in seat 0 and the opponent in seat 1.
	s.first = []game.FirstMaker{game.FirstMakerSeat1, game.FirstMakerSeat0, game.FirstMakerCoinFlip}[choice]
	return s.play, nil
}

// play shows the line-up and plays the chosen game to the end: every
// round, a summary between rounds of a match, and the result. Quitting
// shows the standings so far.
func (s *Session) play(ctx context.Context) (step, error) {
	s.board = NewBoard()
	seats := [2]game.Seat{
		{Name: s.name, Peg: game.PegBlue, Player: &Tapper{s: s, hidden: s.vsHuman}},
		{Name: computerName, Peg: game.PegRed, Player: &pacedComputer{s: s, c: player.NewComputer(s.rng)}},
	}
	if s.vsHuman {
		// Both people share the device, so secrets are made behind
		// handoff cards (D21).
		seats[1] = game.Seat{Name: s.name2, Peg: game.PegRed, Player: &Tapper{s: s, hidden: true}}
	}
	cfg := game.Config{Mode: s.mode, FirstMaker: s.first, Easy: s.easy}
	// NewMatch only fails on a bad setup, which the menus can't produce,
	// so an error here would be a bug. It's still passed up.
	m, err := game.NewMatch(cfg, seats, s.rng, s.board)
	if err != nil {
		return nil, err
	}

	if _, err := s.menu(ctx, modeTitle(s.mode), s.lineup(m), "Start"); err != nil {
		return nil, err
	}

	for !m.Over() {
		res, err := m.PlayRound(ctx)
		if errors.Is(err, ErrQuit) {
			break
		}
		if err != nil {
			return nil, err
		}
		if !m.Over() {
			lines := append(s.roundLines(res, m.Seats()), "Score: "+scoreLine(m.Seats()))
			v := View{Screen: ScreenSummary, Title: fmt.Sprintf("Round %d over", res.Number),
				Lines: lines, Board: s.board.View(nil), Menu: &MenuView{Choices: []string{"Next round"}}}
			s.show(v)
			if _, err := s.choose(ctx, 1); err != nil {
				return nil, err
			}
		}
	}

	title := "Game over"
	if s.mode == game.ModeMatch {
		title = "Match over"
	}
	s.show(View{
		Screen: ScreenResult,
		Title:  title,
		Lines:  s.resultLines(m.Standings()),
		Board:  s.board.View(nil),
		Menu:   &MenuView{Choices: []string{"Play again", "Main menu"}},
	})
	choice, err := s.choose(ctx, 2)
	if err != nil {
		return nil, err
	}
	if choice == 0 {
		return s.play, nil
	}
	return s.mainMenu, nil
}

// lineup describes who plays and who makes the first code, as the
// terminal's line-up does.
func (s *Session) lineup(m *game.Match) []string {
	seats := m.Seats()
	players := "Blue " + seats[0].Name + " vs Red " + seats[1].Name
	first := ""
	if s.first == game.FirstMakerCoinFlip {
		first = "Coin flip: "
	}
	if s.mode == game.ModeMatch {
		return []string{
			fmt.Sprintf("Match: %s. First to %d points wins.", players, game.TargetScore),
			first + seats[m.MakerIdx()].Name + " makes the first code.",
		}
	}
	return []string{"Single game: " + players + ".", first + seats[m.MakerIdx()].Name + " makes the code."}
}

// roundLines describes how a round ended, in the terminal's words.
func (s *Session) roundLines(res game.RoundResult, seats [2]game.Seat) []string {
	breaker := seats[res.BreakerIdx].Name
	if !res.Solved {
		return []string{fmt.Sprintf("%s didn't crack the code in %d rows.", breaker, res.Rows),
			"No points this round."}
	}
	lo, hi := game.Bracket(res.Rows)
	points := fmt.Sprintf("%s scores %d points (solved in rows %d–%d).", breaker, res.Points, lo, hi)
	if s.mode == game.ModeSingle {
		points = fmt.Sprintf("In a match that would score %d points (solved in rows %d–%d).", res.Points, lo, hi)
	}
	return []string{fmt.Sprintf("%s cracked the code in %d %s!", breaker, res.Rows, plural(res.Rows, "row")), points}
}

// resultLines describes how the game ended: the last round and the
// winner, or how far it got before the player quit.
func (s *Session) resultLines(st game.Standings) []string {
	rounds := len(st.Results)
	switch {
	case s.mode == game.ModeSingle && rounds == 0:
		return []string{"Game stopped before the code was cracked."}
	case s.mode == game.ModeSingle:
		return s.roundLines(st.Results[0], st.Seats)
	case st.Winner < 0:
		return []string{fmt.Sprintf("Match stopped after %d %s.", rounds, plural(rounds, "round")),
			"Score: " + scoreLine(st.Seats)}
	default:
		w := st.Seats[st.Winner]
		return append(s.roundLines(st.Results[rounds-1], st.Seats),
			fmt.Sprintf("%s wins the match with %d points after %d %s!", w.Name, w.Score, rounds, plural(rounds, "round")),
			"Final score: "+scoreLine(st.Seats))
	}
}

// scoreLine shows both scores on the 30-point track:
// "Blue Tom 18/30 · Red Computer 10/30". The first player always sits in
// seat 0 with Blue, so Blue always comes first.
func scoreLine(seats [2]game.Seat) string {
	a, b := seats[0], seats[1]
	return fmt.Sprintf("Blue %s %d/%d · Red %s %d/%d", a.Name, a.Score, game.TargetScore, b.Name, b.Score, game.TargetScore)
}

// menu shows a menu screen and returns the 0-based choice.
func (s *Session) menu(ctx context.Context, title string, lines []string, choices ...string) (int, error) {
	s.show(View{Screen: ScreenMenu, Title: title, Lines: lines, Menu: &MenuView{Choices: choices}})
	return s.choose(ctx, len(choices))
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

// pause waits between the computer's guesses, unless the player has
// tapped Skip this round. Skip ends the pause and the rest of the round's
// pauses; Quit returns ErrQuit.
func (s *Session) pause(ctx context.Context) error {
	if s.skipping {
		return nil
	}
	over := s.pace()
	for {
		ev, timeUp, err := s.nextUntil(ctx, over)
		switch {
		case err != nil:
			return err
		case timeUp:
			return nil
		case ev.Kind == KindSkip:
			s.skipping = true
			return nil
		case ev.Kind == KindQuit:
			return ErrQuit
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
// is one or ctx is cancelled.
func (s *Session) next(ctx context.Context) (Event, error) {
	ev, _, err := s.nextUntil(ctx, nil)
	return ev, err
}

// nextUntil is next, but it also stops waiting when over is closed, and
// then reports timeUp. A nil over never closes.
//
// It handles help itself: Help shows the help sheet over the current
// screen, Close shows that screen again, and other events while help is
// open are dropped, since they were taps behind the sheet.
func (s *Session) nextUntil(ctx context.Context, over <-chan struct{}) (ev Event, timeUp bool, err error) {
	for {
		ev, timeUp, err := s.pop(ctx, over)
		if err != nil || timeUp {
			return Event{}, timeUp, err
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
			return ev, false, nil
		}
	}
}

// pop takes the oldest event from the queue, waiting for one if it's
// empty, unless over closes or ctx is cancelled first.
func (s *Session) pop(ctx context.Context, over <-chan struct{}) (Event, bool, error) {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			ev := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return ev, false, nil
		}
		s.mu.Unlock()

		// Go note: select waits for whichever happens first: the doorbell
		// (a new event), the pause ending, or the context being
		// cancelled. Receiving from a nil channel waits forever, so a nil
		// over simply never wins.
		select {
		case <-s.notify:
		case <-over:
			return Event{}, true, nil
		case <-ctx.Done():
			return Event{}, false, ctx.Err()
		}
	}
}

// cleanName trims spaces, uses def for an empty name, and cuts a long one
// to maxName characters.
func cleanName(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
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

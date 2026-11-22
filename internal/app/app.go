package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/input"
	"github.com/tculpepp/codebreaker/internal/player"
	"github.com/tculpepp/codebreaker/internal/ui"
)

// Exit codes returned by ExitCode (implementation-plan.md §4).
const (
	// ExitOK means the game finished normally or the player quit.
	ExitOK = 0
	// ExitError means something went wrong that shouldn't happen.
	ExitError = 1
	// ExitUsage means the command line was wrong (bad flags).
	ExitUsage = 2
	// ExitInterrupted means the player pressed Ctrl-C.
	//
	// Go note: this isn't Go-specific. Unix shells report a program killed
	// by a signal as 128 plus the signal number, and Ctrl-C sends SIGINT,
	// which is signal 2. Programs that stop cleanly on Ctrl-C exit with
	// 130 so scripts can tell what happened.
	ExitInterrupted = 130
)

// Names used for the players.
const (
	defaultName  = "Player"   // used when the first player just presses Enter
	defaultName2 = "Player 2" // used when the second player just presses Enter
	computerName = "Computer" // the computer's seat
)

// nameTakenError reports a second player choosing the first player's name,
// which would make the screens impossible to follow.
type nameTakenError struct {
	name string
}

// Error implements error.
func (e nameTakenError) Error() string {
	return fmt.Sprintf("app: name %q is already taken", e.name)
}

// Friendly returns the message shown to the player (see message.Friendly).
func (e nameTakenError) Friendly() string {
	return e.name + " is already playing. Please choose a different name."
}

// Run plays Codebreaker, reading the player's answers from in and drawing
// on out, until the player quits. o holds the command-line choices (see
// ParseFlags). o.Seed fixes every random choice (the coin flip and the
// computer's codes and guesses), so the same seed and the same answers
// always give the same game.
//
// It returns nil when the player quits or the input ends, the context
// error after Ctrl-C (ctx cancelled), or any other failure. Pass the
// result to ExitCode.
func Run(ctx context.Context, in io.Reader, out io.Writer, o Options) error {
	r := ui.NewRenderer(out, ui.Options{NoColor: o.NoColor, ASCII: o.ASCII})
	return run(ctx, input.NewTerminal(in, out), r, newRNG(o.Seed), o)
}

// ExitCode turns an error from ParseFlags or Run into a process exit code:
// ExitOK for nil or a request for help, ExitUsage for bad flags,
// ExitInterrupted after Ctrl-C, and ExitError for anything else.
func ExitCode(err error) int {
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return ExitOK
	case errors.Is(err, ErrUsage):
		return ExitUsage
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	default:
		return ExitError
	}
}

// newRNG returns the game's random generator for seed.
//
// Go note: math/rand/v2 generators are built from a "source". PCG is a
// small, fast source that takes two 64-bit seed values; using seed for
// both is enough here. The same seed always gives the same sequence.
func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

// run is Run with every dependency passed in, so tests can supply
// scripted input, capture the output, and fix the seed.
//
// Quitting and the end of input count as a normal finish: run says goodbye
// and returns nil. Other errors, including Ctrl-C, are returned as they
// are.
//
// Go note: run is unexported (lowercase), so only this package and its
// tests can call it. The exported Run stays simple for main, while tests
// get full control. Splitting it this way is a common Go pattern.
func run(ctx context.Context, p input.Prompter, r *ui.Renderer, rng *rand.Rand, o Options) error {
	s := &session{p: p, r: r, rng: rng, opts: o}
	err := s.play(ctx)
	if errors.Is(err, input.ErrQuit) || errors.Is(err, io.EOF) {
		err = nil
	}
	if err == nil {
		r.Goodbye()
	}
	return err
}

// state is one screen of the app's flow (implementation-plan.md §5):
//
//	stateName ─▶ stateMainMenu ─▶ stateChooseLevel ─▶ stateChooseOpponent ─person─▶ stateSecondName
//	                 ▲   │                                  │ computer                 │
//	                 │   │                                  ▼                          ▼
//	                 │   │                  Match: statePlay  ·  Single: stateChooseRole ─▶ statePlay
//	                 │   │                                                                     │
//	                 │   └─Quit─▶ stateDone ◀─Quit─ statePlayAgain ◀───────────────────────────┘
//	                 └───────────────New game──────────┘
//
// Go note: this is a "state machine". Each state has its own step method
// that does one screen's work and returns the next state. The main loop
// in play just calls step until stateDone. Compared with nested loops, each
// screen is small, the possible moves are easy to see, and each step can
// be tested on its own.
type state int

// The app's states, in the order a player usually meets them.
const (
	stateName           state = iota // ask the first player's name
	stateMainMenu                    // choose Match, Single game or Quit
	stateChooseLevel                 // Normal or Easy
	stateChooseOpponent              // the computer or another person
	stateSecondName                  // ask the second person's name
	stateChooseRole                  // Single game only: who makes the code
	statePlay                        // show the line-up, confirm, and play
	statePlayAgain                   // offer another game
	stateDone                        // leave the app
)

// session holds what the steps share: the input and screen, the random
// generator, and the choices made so far.
type session struct {
	p       input.Prompter
	r       *ui.Renderer
	rng     *rand.Rand
	name    string      // the first player's name (seat 1, Blue)
	name2   string      // the second person's name (seat 2, Red), in Human vs Human
	vsHuman bool        // whether seat 2 is a person rather than the computer
	cfg     game.Config // the game chosen in the menus
	opts    Options     // command-line choices; each one set skips its menu
}

// play runs the state machine from the welcome screen until stateDone or
// an error. Quitting shows up as an error (input.ErrQuit or io.EOF), which
// run treats as a normal finish.
func (s *session) play(ctx context.Context) error {
	s.r.Welcome()
	st := stateName
	for st != stateDone {
		var err error
		if st, err = s.step(ctx, st); err != nil {
			return err
		}
	}
	return nil
}

// step runs one state and returns the next. An unknown state is a bug, so
// it's reported rather than ignored.
func (s *session) step(ctx context.Context, st state) (state, error) {
	switch st {
	case stateName:
		return s.askName(ctx)
	case stateMainMenu:
		return s.mainMenu(ctx)
	case stateChooseLevel:
		return s.chooseLevel(ctx)
	case stateChooseOpponent:
		return s.chooseOpponent(ctx)
	case stateSecondName:
		return s.askSecondName(ctx)
	case stateChooseRole:
		return s.chooseRole(ctx)
	case statePlay:
		return s.playGame(ctx)
	case statePlayAgain:
		return s.playAgain(ctx)
	default:
		return stateDone, fmt.Errorf("app: unknown state %d", st)
	}
}

// askName asks for the first player's name.
func (s *session) askName(ctx context.Context) (state, error) {
	name, err := s.readName(ctx, s.r.Prompt("Your name (Enter for "+defaultName+")"), defaultName, "")
	if err != nil {
		return stateDone, err
	}
	s.name = name
	return stateMainMenu, nil
}

// askSecondName asks for the second person's name, which must differ from
// the first player's.
func (s *session) askSecondName(ctx context.Context) (state, error) {
	prompt := s.r.Prompt("Second player's name (Enter for " + defaultName2 + ")")
	name, err := s.readName(ctx, prompt, defaultName2, s.name)
	if err != nil {
		return stateDone, err
	}
	s.name2 = name
	if s.cfg.Mode == game.ModeSingle {
		return stateChooseRole, nil
	}
	return statePlay, nil
}

// readName asks for a name until it gets one that isn't taken (ignoring
// case). Pressing Enter gives def; help shows the help and asks again.
func (s *session) readName(ctx context.Context, prompt, def, taken string) (string, error) {
	for {
		name, err := s.p.Line(ctx, prompt)
		if errors.Is(err, input.ErrHelp) {
			s.r.Help()
			continue
		}
		if err != nil {
			return "", err
		}
		if name == "" {
			name = def
		}
		// Go note: strings.EqualFold compares two strings ignoring case,
		// so "Tom" and "TOM" count as the same name.
		if strings.EqualFold(name, taken) {
			s.r.Error(nameTakenError{name: taken})
			continue
		}
		return name, nil
	}
}

// mainMenu offers a match, a Single game, or quitting. The -mode flag
// answers it without asking.
func (s *session) mainMenu(ctx context.Context) (state, error) {
	choice, err := s.chooseUnless(ctx, s.opts.Mode, modes, "Main menu", []string{
		fmt.Sprintf("Match (first to %d points)", game.TargetScore),
		"Single game (one round)",
		"Quit",
	})
	switch {
	case err != nil:
		return stateDone, err
	case choice == 0:
		// The zero Config is a full match with a coin flip (D6).
		s.cfg = game.Config{}
		return stateChooseLevel, nil
	case choice == 1:
		s.cfg = game.Config{Mode: game.ModeSingle}
		return stateChooseLevel, nil
	default:
		return stateDone, nil
	}
}

// chooseLevel asks for the difficulty. Easy mode shows each feedback
// symbol in the same spot as the guess peg it scores; scoring is the same.
// The -difficulty flag answers it without asking.
func (s *session) chooseLevel(ctx context.Context) (state, error) {
	choice, err := s.chooseUnless(ctx, s.opts.Difficulty, levels, "Difficulty:", []string{
		"Normal",
		"Easy (each result symbol shows which peg it's for)",
	})
	if err != nil {
		return stateDone, err
	}
	s.cfg.Easy = choice == 1
	return stateChooseOpponent, nil
}

// chooseOpponent asks whether to play the computer or another person at
// the same keyboard. Computer vs Computer isn't offered (D17). The
// -opponent flag answers it without asking.
func (s *session) chooseOpponent(ctx context.Context) (state, error) {
	choice, err := s.chooseUnless(ctx, s.opts.Opponent, opponents,
		"Play against:", []string{"The computer", "Another person"})
	switch {
	case err != nil:
		return stateDone, err
	case choice == 1:
		s.vsHuman = true
		return stateSecondName, nil
	}
	s.vsHuman = false
	if s.cfg.Mode == game.ModeSingle {
		return stateChooseRole, nil
	}
	return statePlay, nil
}

// chooseRole asks who makes the code in a Single game (D13). The first
// player always sits in seat 0 and the opponent in seat 1. The -role flag
// answers it without asking.
func (s *session) chooseRole(ctx context.Context) (state, error) {
	choices := []string{
		"Break the computer's code",
		"Make a code for the computer to break",
		"Flip a coin",
	}
	if s.vsHuman {
		choices[0] = s.name + " breaks " + s.name2 + "'s code"
		choices[1] = s.name2 + " breaks " + s.name + "'s code"
	}
	choice, err := s.chooseUnless(ctx, s.opts.Role, roles, "Single game: who makes the code?", choices)
	if err != nil {
		return stateDone, err
	}
	// Go note: an array literal with indexes, like the one below, works as
	// a small lookup table from a menu choice to a value. choice is always
	// 0, 1 or 2 here, because choose only returns listed choices.
	firstMaker := [...]game.FirstMaker{
		0: game.FirstMakerSeat1, // the opponent makes, so the first player breaks
		1: game.FirstMakerSeat0, // the first player makes
		2: game.FirstMakerCoinFlip,
	}[choice]
	s.cfg.FirstMaker = firstMaker
	return statePlay, nil
}

// playGame seats the players, shows the line-up and the coin flip, waits
// for the player to start, and plays the game chosen in the menus. The
// renderer, as the match's Observer, draws everything as it happens.
func (s *session) playGame(ctx context.Context) (state, error) {
	seats := [2]game.Seat{
		{Name: s.name, Peg: game.PegBlue, Player: player.NewHuman(s.p, s.r)},
		{Name: computerName, Peg: game.PegRed, Player: player.NewComputer(s.rng)},
	}
	if s.vsHuman {
		// Both people share the keyboard and screen, so both type their
		// secrets hidden and hand the keyboard over (D10).
		first := player.NewHuman(s.p, s.r)
		second := player.NewHuman(s.p, s.r)
		first.Hidden, second.Hidden = true, true
		seats[0].Player = first
		seats[1] = game.Seat{Name: s.name2, Peg: game.PegRed, Player: second}
	}
	// NewMatch only fails on a bad setup, and the menus can't produce one,
	// so an error here would be a bug. It's still passed up, not ignored.
	m, err := game.NewMatch(s.cfg, seats, s.rng, s.r)
	if err == nil {
		err = s.startAndPlay(ctx, m)
	}
	if err != nil {
		return stateDone, err
	}
	return statePlayAgain, nil
}

// startAndPlay shows who plays and who makes the first code, asks the
// player to press Enter (requirements.md: "Confirm the start and show the
// coin-flip result"), then plays the match to the end.
func (s *session) startAndPlay(ctx context.Context, m *game.Match) error {
	coinFlip := s.cfg.FirstMaker == game.FirstMakerCoinFlip
	s.r.Setup(s.cfg.Mode, m.Seats(), m.MakerIdx(), coinFlip)
	for {
		_, err := s.p.Line(ctx, s.r.Prompt("Press Enter to start"))
		if errors.Is(err, input.ErrHelp) {
			s.r.Help()
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	_, err := m.Play(ctx)
	return err
}

// playAgain offers another game, which goes back to the main menu.
func (s *session) playAgain(ctx context.Context) (state, error) {
	choice, err := s.choose(ctx, "Play again?", []string{"New game", "Quit"})
	switch {
	case err != nil:
		return stateDone, err
	case choice == 0:
		return stateMainMenu, nil
	default:
		return stateDone, nil
	}
}

// chooseUnless returns the menu choice that answer (a command-line flag's
// value) stands for: its index in values, which lists the flag values in
// the same order as choices. If answer is "", it asks with choose instead.
func (s *session) chooseUnless(ctx context.Context, answer string, values []string,
	title string, choices []string) (int, error) {
	// Go note: slices.Index returns the position of a value in a slice, or
	// -1 if it isn't there. ParseFlags has already checked the value, so
	// it's always found.
	if answer != "" {
		return slices.Index(values, answer), nil
	}
	return s.choose(ctx, title, choices)
}

// choose shows a menu and returns the 0-based index of the choice made. A
// bad answer is explained and asked again; help is shown followed by the
// menu again, so the choices stay in view.
func (s *session) choose(ctx context.Context, title string, choices []string) (int, error) {
	s.r.Menu(title, choices)
	prompt := s.r.Prompt("Choose " + s.r.Range(1, len(choices)))
	for {
		line, err := s.p.Line(ctx, prompt)
		if errors.Is(err, input.ErrHelp) {
			s.r.Help()
			s.r.Menu(title, choices)
			continue
		}
		if err != nil {
			return 0, err
		}
		choice, err := ui.ParseChoice(line, len(choices))
		if err != nil {
			s.r.Error(err)
			continue
		}
		return choice, nil
	}
}

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"

	"codebreaker/internal/game"
	"codebreaker/internal/input"
	"codebreaker/internal/player"
	"codebreaker/internal/ui"
)

// Exit codes returned by ExitCode (implementation-plan.md §4).
const (
	// ExitOK means the game finished normally or the player quit.
	ExitOK = 0
	// ExitError means something went wrong that shouldn't happen.
	ExitError = 1
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
	defaultName  = "Player"   // used when the player just presses Enter
	computerName = "Computer" // the computer's seat
)

// Run plays Codebreaker, reading the player's answers from in and drawing
// on out, until the player quits. seed fixes every random choice (the
// coin flip and the computer's codes and guesses), so the same seed and
// the same answers always give the same game.
//
// It returns nil when the player quits or the input ends, the context
// error after Ctrl-C (ctx cancelled), or any other failure. Pass the
// result to ExitCode.
func Run(ctx context.Context, in io.Reader, out io.Writer, seed uint64) error {
	return run(ctx, input.NewTerminal(in, out), ui.NewRenderer(out), newRNG(seed))
}

// ExitCode turns Run's result into a process exit code: ExitOK for nil,
// ExitInterrupted after Ctrl-C, and ExitError for anything else.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitOK
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
func run(ctx context.Context, p input.Prompter, r *ui.Renderer, rng *rand.Rand) error {
	s := &session{p: p, r: r, rng: rng}
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
//	stateName ──▶ stateMainMenu ──Match───────────────▶ statePlay
//	                   │  ▲      └─Single─▶ stateChooseRole ──▶ statePlay
//	                   │  └────New game── statePlayAgain ◀───────────┘
//	                   └──Quit──▶ stateDone ◀──Quit──┘
//
// Go note: this is a "state machine". Each state has its own step method
// that does one screen's work and returns the next state. The main loop
// in play just calls step until stateDone. Compared with nested loops, each
// screen is small, the possible moves are easy to see, and each step can
// be tested on its own.
type state int

// The app's states, in the order a player usually meets them.
const (
	stateName       state = iota // ask the player's name
	stateMainMenu                // choose Match, Single game or Quit
	stateChooseRole              // Single game only: Maker, Breaker or coin flip
	statePlay                    // show the line-up, confirm, and play
	statePlayAgain               // offer another game
	stateDone                    // leave the app
)

// session holds what the steps share: the input and screen, the random
// generator, and the choices made so far.
type session struct {
	p    input.Prompter
	r    *ui.Renderer
	rng  *rand.Rand
	name string      // the human player's name
	cfg  game.Config // the game chosen in the menus
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

// askName asks for the player's name. Pressing Enter gives defaultName.
func (s *session) askName(ctx context.Context) (state, error) {
	for {
		name, err := s.p.Line(ctx, " Your name (Enter for "+defaultName+") ▸ ")
		if errors.Is(err, input.ErrHelp) {
			s.r.Help()
			continue
		}
		if err != nil {
			return stateDone, err
		}
		if name == "" {
			name = defaultName
		}
		s.name = name
		return stateMainMenu, nil
	}
}

// mainMenu offers a match, a Single game, or quitting. Playing against
// another person arrives in v0.5.0.
func (s *session) mainMenu(ctx context.Context) (state, error) {
	choice, err := s.choose(ctx, "Main menu", []string{
		fmt.Sprintf("Match against the computer (first to %d points)", game.TargetScore),
		"Single game against the computer",
		"Quit",
	})
	switch {
	case err != nil:
		return stateDone, err
	case choice == 0:
		// The zero Config is a full match with a coin flip (D6).
		s.cfg = game.Config{}
		return statePlay, nil
	case choice == 1:
		return stateChooseRole, nil
	default:
		return stateDone, nil
	}
}

// chooseRole asks which role the human takes in a Single game (D13). The
// human always sits in seat 0 and the computer in seat 1.
func (s *session) chooseRole(ctx context.Context) (state, error) {
	choice, err := s.choose(ctx, "Single game: which role?", []string{
		"Break the computer's code",
		"Make a code for the computer to break",
		"Flip a coin",
	})
	if err != nil {
		return stateDone, err
	}
	// Go note: an array literal with indexes, like the one below, works as
	// a small lookup table from a menu choice to a value. choice is always
	// 0, 1 or 2 here, because choose only returns listed choices.
	firstMaker := [...]game.FirstMaker{
		0: game.FirstMakerSeat1, // the computer makes, so the human breaks
		1: game.FirstMakerSeat0, // the human makes
		2: game.FirstMakerCoinFlip,
	}[choice]
	s.cfg = game.Config{Mode: game.ModeSingle, FirstMaker: firstMaker}
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
		_, err := s.p.Line(ctx, " Press Enter to start ▸ ")
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

// choose shows a menu and returns the 0-based index of the choice made. A
// bad answer is explained and asked again; help is shown followed by the
// menu again, so the choices stay in view.
func (s *session) choose(ctx context.Context, title string, choices []string) (int, error) {
	s.r.Menu(title, choices)
	prompt := fmt.Sprintf(" Choose 1–%d ▸ ", len(choices))
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

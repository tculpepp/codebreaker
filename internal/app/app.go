package app

import (
	"context"
	"errors"
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
// on out, until the player quits. seed fixes the computer's choices, so
// the same seed and the same answers always give the same game.
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
	err := play(ctx, p, r, rng)
	if errors.Is(err, input.ErrQuit) || errors.Is(err, io.EOF) {
		err = nil
	}
	if err == nil {
		r.Goodbye()
	}
	return err
}

// play runs the session: welcome, ask for a name, then play Single games
// until the player declines another. It returns nil only when the player
// chooses Quit in the menu; otherwise it returns whatever stopped it.
func play(ctx context.Context, p input.Prompter, r *ui.Renderer, rng *rand.Rand) error {
	r.Welcome()
	name, err := askName(ctx, p, r)
	if err != nil {
		return err
	}
	for {
		if err := singleGame(ctx, name, p, r, rng); err != nil {
			return err
		}
		again, err := askPlayAgain(ctx, p, r)
		if err != nil || !again {
			return err
		}
	}
}

// askName asks for the player's name. Pressing Enter gives defaultName.
func askName(ctx context.Context, p input.Prompter, r *ui.Renderer) (string, error) {
	for {
		name, err := p.Line(ctx, " Your name (Enter for "+defaultName+") ▸ ")
		if errors.Is(err, input.ErrHelp) {
			r.Help()
			continue
		}
		if err != nil {
			return "", err
		}
		if name == "" {
			name = defaultName
		}
		return name, nil
	}
}

// singleGame plays one Single game: the computer makes a code and the
// human tries to break it. The renderer, as the match's Observer, draws
// everything as it happens.
func singleGame(ctx context.Context, name string, p input.Prompter, r *ui.Renderer, rng *rand.Rand) error {
	seats := [2]game.Seat{
		{Name: name, Peg: game.PegBlue, Player: player.NewHuman(p, r)},
		{Name: computerName, Peg: game.PegRed, Player: player.NewComputer(rng)},
	}
	// The computer always makes the code in this version (it can't break
	// codes until v0.4.0), so seat 1 is the first and only Maker.
	cfg := game.Config{Mode: game.ModeSingle, FirstMaker: game.FirstMakerSeat1}

	// NewMatch only fails on a bad setup, and this setup is fixed, so an
	// error here would be a bug. It's still passed up rather than ignored.
	m, err := game.NewMatch(cfg, seats, rng, r)
	if err == nil {
		_, err = m.Play(ctx)
	}
	return err
}

// askPlayAgain shows the play-again menu and reports whether the player
// chose another game.
func askPlayAgain(ctx context.Context, p input.Prompter, r *ui.Renderer) (bool, error) {
	choices := []string{"New game", "Quit"}
	r.Menu("Play again?", choices)
	for {
		line, err := p.Line(ctx, " Choose 1–2 ▸ ")
		if errors.Is(err, input.ErrHelp) {
			r.Help()
			r.Menu("Play again?", choices)
			continue
		}
		if err != nil {
			return false, err
		}
		choice, err := ui.ParseChoice(line, len(choices))
		if err != nil {
			r.Error(err)
			continue
		}
		return choice == 0, nil
	}
}

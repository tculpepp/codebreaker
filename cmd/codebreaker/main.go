// Command codebreaker is a terminal version of the Code Breaker board
// game. Run it and follow the prompts; type ? for help or q to quit.
//
// main is kept tiny on purpose: it sets up Ctrl-C handling, runs the game,
// and exits with the right code. All the logic lives in package app, where
// it can be tested.
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"

	"codebreaker/internal/app"
)

// main runs the game on the terminal and exits with app.ExitCode.
func main() {
	// Go note: signal.NotifyContext returns a context that is cancelled
	// when the program receives Ctrl-C (os.Interrupt). Every prompt
	// watches this context, so Ctrl-C stops the game cleanly wherever it
	// is. Calling stop restores the default Ctrl-C behavior, so a second
	// Ctrl-C would kill the program at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	// Go note: rand.Uint64 (the top-level function) uses a generator that
	// Go seeds randomly at start-up, so each run gets a new seed. The
	// game itself only uses the seeded generator built from it.
	err := app.Run(ctx, os.Stdin, os.Stdout, rand.Uint64())
	stop()

	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "codebreaker:", err)
	}
	// Go note: os.Exit ends the program immediately, without running
	// deferred functions. That's why stop is called above rather than
	// deferred: a `defer stop()` would never run.
	os.Exit(app.ExitCode(err))
}

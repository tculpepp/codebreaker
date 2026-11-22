// Command codebreaker is a terminal version of the Code Breaker board
// game. Run it and follow the prompts; type ? for help or q to quit.
//
// Usage:
//
//	codebreaker [-mode match|single] [-opponent computer|human]
//	            [-role breaker|maker|random] [-seed N] [-no-color] [-ascii]
//
// Each flag answers a menu question, which is then skipped. Run
// `codebreaker -h` for the full list.
//
// main is kept tiny on purpose: it reads the flags, sets up Ctrl-C
// handling, runs the game, and exits with the right code. All the logic
// lives in package app, where it can be tested.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/tculpepp/codebreaker/internal/app"
)

// main runs the game on the terminal and exits with app.ExitCode.
func main() {
	// Bad flags print their own message and the usage text, then exit 2;
	// -h prints the usage and exits 0.
	opts, err := app.ParseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(app.ExitCode(err))
	}

	// Go note: signal.NotifyContext returns a context that is cancelled
	// when the program receives Ctrl-C (os.Interrupt). Every prompt
	// watches this context, so Ctrl-C stops the game cleanly wherever it
	// is. Calling stop restores the default Ctrl-C behavior, so a second
	// Ctrl-C would kill the program at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	err = app.Run(ctx, os.Stdin, os.Stdout, opts)
	stop()

	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "codebreaker:", err)
	}
	// Go note: os.Exit ends the program immediately, without running
	// deferred functions. That's why stop is called above rather than
	// deferred: a `defer stop()` would never run.
	os.Exit(app.ExitCode(err))
}

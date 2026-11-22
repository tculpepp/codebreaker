// Package app wires the other packages together into the game a person
// plays: it asks for a name, offers a menu, seats the players, runs the
// chosen game, and offers another.
//
// # Where it fits
//
// app is the top of the program. cmd/codebreaker (main) only sets up
// Ctrl-C handling, calls Run, and turns the result into an exit code with
// ExitCode. Everything else happens here, where it can be tested with
// scripted input and a seeded random generator.
//
// The screens form a small state machine (see the state type): name, main
// menu, role choice for a Single game, play, and play again. This version
// plays against the computer only, as a full match or a Single game in
// either role. Human vs Human arrives in v0.5.0, and command-line flags in
// v1.0.0.
//
// # Go concepts shown here
//
//   - Keeping main small so the real logic is testable
//   - A state machine: a named int type and one step method per state
//   - An unexported run function that takes its dependencies as arguments
//   - Process exit codes, including 130 for Ctrl-C
//   - Embedding an interface in a struct to wrap it (in the tests)
//
// Search for "Go note:" to find the explanation of each one.
package app

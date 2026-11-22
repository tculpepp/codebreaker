// Package app wires the other packages together into the game a person
// plays: it asks for a name, seats the players, runs a game, and offers
// another.
//
// # Where it fits
//
// app is the top of the program. cmd/codebreaker (main) only sets up
// Ctrl-C handling, calls Run, and turns the result into an exit code with
// ExitCode. Everything else happens here, where it can be tested with
// scripted input and a seeded random generator.
//
// This version plays one kind of game: a Single game in which the human
// breaks the computer's code. Choosing the mode and the opponent, flags,
// and the full menu (as a small state machine) arrive in later versions.
//
// # Go concepts shown here
//
//   - Keeping main small so the real logic is testable
//   - An unexported run function that takes its dependencies as arguments
//   - Process exit codes, including 130 for Ctrl-C
//   - Embedding an interface in a struct to wrap it (in the tests)
//
// Search for "Go note:" to find the explanation of each one.
package app

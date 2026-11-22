// Package game holds the state and rules of play: a Round (one secret, up to
// 12 guesses), scoring by rows used, and a Match that swaps roles between
// two seats until someone reaches 30 points.
//
// # Where it fits
//
// game sits on top of package code (colors, guesses, scoring a guess) and
// below everything with I/O (input, ui, player, app). It never reads the
// keyboard or prints anything. Players and the screen connect through small
// interfaces declared here (CodeMaker, CodeBreaker, Observer), so the whole
// game can be tested with fake players and no terminal.
//
// # Go concepts shown here
//
//   - Encapsulation with unexported struct fields (the secret stays private)
//   - Pointer receivers for methods that change state
//   - Interfaces declared by the package that uses them
//   - Interface embedding (Participant = CodeMaker + CodeBreaker)
//   - Sentinel errors with errors.Is, wrapping with %w
//   - Returning copies so callers can't change internal state
//   - Enums with iota plus a String method
//   - context.Context for cancellation
//   - Test doubles (fakes) that satisfy interfaces
//
// Search for "Go note:" to find the explanation of each one.
package game

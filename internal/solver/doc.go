// Package solver is the computer's Code Breaker. It keeps a list of every
// code that could still be the secret and, after each guess, keeps only the
// codes that would have given the same feedback. Its next guess is a random
// code from what's left.
//
// This "random consistent guess" strategy averages about 5 to 6 rows, and
// a test confirms it cracks all 6720 possible codes within 12 rows. Smarter
// strategies (Knuth's minimax) are out of scope.
//
// # Where it fits
//
// solver depends only on package code. It never sees the secret: it only
// learns the feedback for its own guesses, through Update. player.Computer
// wraps it to play the Code Breaker role in a game.
//
// # Go concepts shown here
//
//   - Filtering a slice in place, reusing its memory
//   - An injected *rand.Rand (math/rand/v2) for reproducible choices
//   - Deterministic tests of random behavior, using fixed seeds
//   - testing.Short to skip slow work in quick test runs
//   - Benchmarks with b.Loop
//
// Search for "Go note:" to find the explanation of each one.
package solver

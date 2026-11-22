// Package ui draws the game in the terminal: the board, the scores, the
// end-of-round and end-of-match summaries, menus, help, and error
// messages (worded by package message).
//
// # Where it fits
//
// ui reads from game and code but never changes them. Its Renderer
// implements game.Observer, so a Match tells it when a round starts, when
// a turn is played and when things end, and the Renderer draws the right
// screen. Players don't draw the board themselves; they only prompt and
// report mistakes. That keeps the board from being drawn twice.
//
// In a terminal, the Renderer uses color and redraws the board in place
// after each turn. When the output is piped to a file, or color is turned
// off (Options, NO_COLOR, TERM=dumb), it draws plain text that is simply
// appended. Options.ASCII swaps every symbol for an ASCII one. On Windows,
// vt_windows.go turns on escape-code support in the console first.
//
// # Go concepts shown here
//
//   - strings.Builder for building output efficiently
//   - Writing to any io.Writer, so tests can capture the output
//   - fmt widths, which count runes (characters) rather than bytes
//   - Golden-file tests and a custom -update test flag
//   - ANSI escape codes, and detecting a terminal with a type assertion
//   - regexp, compiled once in a package-level variable
//   - Build constraints (//go:build) for Windows-only code
//   - t.Setenv for tests that depend on environment variables
//
// Search for "Go note:" to find the explanation of each one.
package ui

// Package ui draws the game in the terminal: the board, the scores, the
// end-of-round and end-of-match summaries, menus, help, and friendly error
// messages.
//
// # Where it fits
//
// ui reads from game and code but never changes them. Its Renderer
// implements game.Observer, so a Match tells it when a round starts, when
// a turn is played and when things end, and the Renderer draws the right
// screen. Players don't draw the board themselves; they only prompt and
// report mistakes. That keeps the board from being drawn twice.
//
// This version draws plain text that is simply appended to the output, so
// it works in any terminal and when piped to a file. Colors, clearing the
// screen, an ASCII-only mode and Windows support arrive in v1.0.0.
//
// # Go concepts shown here
//
//   - strings.Builder for building output efficiently
//   - Writing to any io.Writer, so tests can capture the output
//   - errors.As for picking out an error's concrete type
//   - fmt widths, which count runes (characters) rather than bytes
//   - Golden-file tests and a custom -update test flag
//
// Search for "Go note:" to find the explanation of each one.
package ui

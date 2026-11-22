// Package input reads what the player types. Its Prompter shows a prompt,
// waits for one line, and turns the quit and help commands into errors
// (ErrQuit, ErrHelp) so every prompt in the game supports them for free.
//
// # Where it fits
//
// input knows nothing about the game. It sits below player and app, which
// call Line whenever they need an answer. Tests use NewScripted to feed in
// a fixed list of lines, so whole games run without a keyboard.
//
// Secret reads a code without showing it, for a human Code Maker in
// Human vs Human. It uses golang.org/x/term when the input is a real
// terminal, and reads an ordinary line otherwise.
//
// # Go concepts shown here
//
//   - io.Reader and io.Writer, Go's small interfaces for any byte stream
//   - bufio.Scanner for reading line by line
//   - Goroutines, channels and select
//   - context.Context cancellation (Ctrl-C while waiting for input)
//   - sync.Once for starting something exactly once
//   - Closing a channel, and the "comma ok" receive that detects it
//   - Type assertions, and the nil-interface trap
//   - defer for cleanup that must always happen (restoring the terminal)
//
// Search for "Go note:" to find the explanation of each one.
package input

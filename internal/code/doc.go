// Package code holds the pure rules of Code Breaker: the 8 colors, the
// 5-peg secret code, parsing a typed guess, validation, and scoring a guess
// against the secret (the ● and △ feedback).
//
// # Where it fits
//
// This is the bottom layer of the architecture (docs/implementation-plan.md
// §2). It imports nothing from the rest of the project and does no I/O, so
// every function can be tested quickly and on its own. All other packages
// build on it.
//
// # Go concepts shown here
//
//   - Named types built on basic types (Color is a uint8)
//   - iota for numbering constants
//   - Methods with value receivers
//   - The fmt.Stringer interface, satisfied without declaring it
//   - Arrays vs slices (Code is a fixed-size array)
//   - Custom error types and errors.As
//   - Bit tricks with math/bits
//   - Lazy one-time setup with sync.OnceValue
//   - Table-driven tests, subtests, fuzz tests and benchmarks
//
// Search for "Go note:" to find the explanation of each one.
//
// Go note: a package comment like this one goes right above the `package`
// line in exactly one file. By convention that file is doc.go. `go doc` and
// pkg.go.dev show it as the package's introduction.
package code

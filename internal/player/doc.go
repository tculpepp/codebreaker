// Package player holds the two kinds of player: Human, who types codes at
// the keyboard, and Computer. Both satisfy game.Participant, so a Match can
// seat either kind in either role.
//
// # Where it fits
//
// player sits between game and the outside world. A Human gets its answers
// from an input.Prompter and reports mistakes to a Screen (in the real game,
// the ui.Renderer). It checks every guess itself, so a typo never costs a
// row. A Computer makes random codes and breaks codes with package solver.
// It uses an injected random generator, so a seeded game plays the same way
// every time.
//
// # Go concepts shown here
//
//   - Satisfying an interface implicitly (no "implements" keyword)
//   - Compile-time interface checks: var _ game.Participant = (*Human)(nil)
//   - Small interfaces declared by the code that needs them (Screen)
//   - Composition: a Human is built from a Prompter and a Screen
//   - Function values, passed in to vary one step of a shared loop
//
// Search for "Go note:" to find the explanation of each one.
package player

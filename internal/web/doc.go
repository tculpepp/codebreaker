// Package web is the logic of the browser version of Codebreaker: what
// each screen shows, and what the player's taps and key presses do. It
// has no browser code at all, so it's tested with plain `go test`.
//
// # Where it fits
//
// The browser version is a second front end beside the terminal one
// (docs/web-plan.md). Both use the same rules packages: code, game,
// solver and player.Computer. This package plays the part that ui and app
// play for the terminal:
//
//	page (render.js) ──Event──▶ bridge ──▶ web ──▶ game, code, solver
//	page (render.js) ◀──View─── bridge ◀── web
//
// Go decides everything and describes each screen as a View, which is
// sent to the page as JSON. The page's JavaScript only draws the View and
// sends back Events (D23). The thin bridge package, the only Go code that
// talks to the browser, connects the two.
//
// It never imports the terminal packages input, ui or app, and they never
// import it, so neither front end can break the other.
//
// # Pieces
//
//   - View and its parts (view.go): everything the page draws, as JSON.
//   - Event (event.go): one thing the player did, parsed from JSON.
//   - Draft (draft.go): the row of pegs being built before Submit.
//   - Board (board.go): a game.Observer that keeps the board for the View.
//
// # Go concepts shown here
//
//   - Struct tags for encoding/json, and omitempty
//   - Pointer fields for optional parts of a JSON object
//   - String-based enums for values that travel as text
//   - Unexported fields that encoding/json skips
//   - Golden JSON files as the contract with another language
//
// Search for "Go note:" to find the explanation of each one.
package web

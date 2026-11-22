//go:build js && wasm

// Command web is the browser version of Codebreaker, compiled to
// WebAssembly. Build it with `make web` and try it with `make serve`; see
// docs/web-plan.md.
//
// Like cmd/codebreaker, main is kept tiny: it makes a session, connects
// it to the page, and runs it. All the logic lives in package web.
package main

import (
	"context"
	"math/rand/v2"
	"time"

	"codebreaker/internal/web"
	"codebreaker/internal/web/bridge"
)

// main runs the game until the page is closed.
//
// Go note: in WebAssembly, the program ends when main returns, and then
// the page's calls into Go fail. Run never returns here (its context is
// never cancelled), so main keeps the program alive. While Run waits for
// an event, Go hands control back to the browser, so the page stays
// responsive.
func main() {
	// A fresh seed every visit, so every game is different.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	// The computer pauses this long before each guess, so its rows can be
	// watched (Web App Requirement 5).
	s := web.NewSession(rng, web.Delay(700*time.Millisecond), bridge.Render)
	release := bridge.Listen(s)
	defer release()
	// Run only ever returns with an error, so reaching the next line
	// means something went wrong. Say so on the page rather than leave
	// it frozen.
	err := s.Run(context.Background())
	bridge.Render(web.View{Title: "Something went wrong", Lines: []string{err.Error()}})
}

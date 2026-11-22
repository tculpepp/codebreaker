//go:build js && wasm

// Package bridge connects a web.Session to the browser page. It's the only
// Go code in the project that talks to JavaScript, and it's kept as thin as
// possible: everything worth testing lives in package web, which has no
// browser code.
//
// # The two directions
//
//   - Go to page: Render encodes a web.View as JSON and calls the page's
//     codebreakerRender function (defined in render.js).
//   - Page to Go: Listen defines a global codebreakerSend function. The
//     page calls it with an event's JSON for every tap or key press, and
//     it's passed to the session.
//
// # Go concepts shown here
//
//   - syscall/js: calling JavaScript, and letting JavaScript call Go
//   - js.FuncOf callbacks, which must never block
//   - Build constraints: this package only exists for js/wasm builds
//   - A package variable swapped in tests (logError)
//
// Search for "Go note:" to find the explanation of each one.
//
// Go note: the //go:build line at the top of each file means these files
// are only compiled when GOOS=js and GOARCH=wasm. A normal `go build ./...`
// or `go test ./...` skips this package entirely; `make test-wasm` runs its
// tests under Node.js.
package bridge

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/tculpepp/codebreaker/internal/web"
)

// Render draws v on the page by passing it, as JSON, to the page's
// codebreakerRender function. Pass it to web.NewSession as the show
// function.
//
// Go note: js.Global() is JavaScript's global object (window, in a
// browser). Call looks up a function on it by name and calls it. Go
// strings are converted to JavaScript strings automatically.
//
// If the page's drawing code throws, the error is logged to the console and
// the game carries on: the next View may well draw fine.
func Render(v web.View) {
	// Go note: when JavaScript called through syscall/js throws, Call
	// panics with a js.Error. An unrecovered panic would end the whole
	// program and freeze the page, so recover turns it back into a log
	// message. recover only works inside a deferred function.
	defer func() {
		if r := recover(); r != nil {
			logError(fmt.Sprint("drawing the screen failed: ", r))
		}
	}()
	// A View is plain data, so encoding it can't fail.
	data, _ := json.Marshal(v)
	js.Global().Call("codebreakerRender", string(data))
}

// Listen defines the global codebreakerSend function, which the page calls
// with each event's JSON, and passes every valid event to s. A bad event
// is a bug in the page, so it's logged to the console and dropped. Call
// the returned function to remove codebreakerSend again.
func Listen(s *web.Session) (release func()) {
	// Go note: js.FuncOf wraps a Go function so JavaScript can call it.
	// While it runs, the page is frozen: JavaScript waits for it to
	// return. So it must never block. It only parses the event and hands
	// it to s.Handle, which never waits.
	send := js.FuncOf(func(this js.Value, args []js.Value) any {
		ev, err := web.ParseEvent([]byte(args[0].String()))
		if err != nil {
			logError(err.Error())
			return nil
		}
		s.Handle(ev)
		return nil
	})
	js.Global().Set("codebreakerSend", send)
	return func() {
		js.Global().Delete("codebreakerSend")
		// Go note: a js.Func holds on to the Go function until Release is
		// called, so the garbage collector can't free it before then.
		send.Release()
	}
}

// logError reports a problem in the browser's console. Tests replace it to
// see what's logged.
var logError = consoleError

// consoleError writes msg to the browser's console as an error.
func consoleError(msg string) {
	js.Global().Get("console").Call("error", "codebreaker: "+msg)
}

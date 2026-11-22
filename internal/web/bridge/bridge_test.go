//go:build js && wasm

package bridge

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"codebreaker/internal/web"
)

// fakePage stands in for render.js: it defines codebreakerRender, which
// records each View's JSON. The tests run under Node.js, which has the same
// global object as a browser page, minus the page itself.
func fakePage(t *testing.T) chan string {
	t.Helper()
	got := make(chan string, 10)
	render := js.FuncOf(func(this js.Value, args []js.Value) any {
		got <- args[0].String()
		return nil
	})
	js.Global().Set("codebreakerRender", render)
	t.Cleanup(func() {
		js.Global().Delete("codebreakerRender")
		render.Release()
	})
	return got
}

// TestRender checks that a View reaches the page as JSON.
func TestRender(t *testing.T) {
	got := fakePage(t)
	Render(web.View{Screen: web.ScreenName, Title: "Codebreaker"})
	if s := <-got; s != `{"screen":"name","title":"Codebreaker","input":{}}` {
		t.Errorf("page received %s", s)
	}
}

// TestListen checks the other direction: the page calls codebreakerSend
// with an event's JSON, and the session acts on it. A bad event is logged,
// not passed on.
func TestListen(t *testing.T) {
	got := fakePage(t)
	var logged []string
	logError = func(msg string) { logged = append(logged, msg) }
	t.Cleanup(func() { logError = consoleError })

	s := web.NewSession(rand.New(rand.NewPCG(1, 1)), Render)
	release := Listen(s)
	defer release()
	go s.Run(t.Context())

	if v := decode(t, next(t, got)); v.Screen != web.ScreenName {
		t.Fatalf("first screen = %q", v.Screen)
	}
	send := js.Global().Get("codebreakerSend")
	send.Invoke(`{"kind":"dance"}`)
	send.Invoke(`{"kind":"text","text":"Tom"}`)
	if v := decode(t, next(t, got)); v.Screen != web.ScreenMenu || !strings.Contains(v.Lines[0], "Tom") {
		t.Errorf("after the name: %+v", v)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "unknown kind") {
		t.Errorf("logged %q, want one bad-event message", logged)
	}

	release()
	if !js.Global().Get("codebreakerSend").IsUndefined() {
		t.Error("codebreakerSend still defined after release")
	}
}

// TestConsoleError checks that the default logger writes to the console
// without failing.
func TestConsoleError(t *testing.T) {
	consoleError("bridge test: this message is expected")
}

// next waits for the page to receive a View.
func next(t *testing.T, got chan string) string {
	t.Helper()
	select {
	case s := <-got:
		return s
	case <-time.After(time.Second):
		t.Fatal("no View rendered within a second")
		return ""
	}
}

// decode reads a View's JSON back, so the test can check its fields.
func decode(t *testing.T, s string) web.View {
	t.Helper()
	var v web.View
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad JSON %s: %v", s, err)
	}
	return v
}

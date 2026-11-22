package web

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// harness runs a Session in the background, the way the browser does, and
// lets a test send events and read the Views it shows.
type harness struct {
	t      *testing.T
	s      *Session
	views  chan View
	cancel context.CancelFunc
	done   chan error // Run's result, once it returns
}

// seed is the seed every session test uses, so the computer's secret is
// known (see secretFor).
const seed = 1

// newRNG is the generator a session is given in these tests.
func newRNG() *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

// secretFor predicts the computer's first secret: it's the first thing
// the session's generator is used for, so a twin generator gives the same
// code.
func secretFor() code.Code { return code.Random(newRNG()) }

// start runs a new session and returns the harness. The session is
// stopped when the test ends.
func start(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, views: make(chan View, 100), done: make(chan error, 1)}
	// Go note: the show function runs on the session's goroutine. Sending
	// each View on a buffered channel hands it to the test's goroutine
	// safely; the race detector would catch a shared variable instead.
	h.s = NewSession(newRNG(), func(v View) { h.views <- v })
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- h.s.Run(ctx) }()
	t.Cleanup(cancel)
	return h
}

// next returns the next View the session shows, failing the test if none
// comes within a second.
func (h *harness) next() View {
	h.t.Helper()
	select {
	case v := <-h.views:
		return v
	case <-time.After(time.Second):
		h.t.Fatal("no View shown within a second")
		return View{}
	}
}

// send passes events to the session, as taps on the page would.
func (h *harness) send(evs ...Event) {
	for _, ev := range evs {
		h.s.Handle(ev)
	}
}

// expect returns the next View, failing the test unless it's on screen.
func (h *harness) expect(screen Screen) View {
	h.t.Helper()
	v := h.next()
	if v.Screen != screen {
		h.t.Fatalf("screen = %q, want %q (view %+v)", v.Screen, screen, v)
	}
	return v
}

// quiet checks that nothing more is shown for a short while: the events
// sent so far were ignored.
func (h *harness) quiet() {
	h.t.Helper()
	select {
	case v := <-h.views:
		h.t.Fatalf("unexpected View %+v", v)
	case <-time.After(50 * time.Millisecond):
	}
}

// Event helpers, so scripts read as what the player did.
func colorEv(letter string) Event {
	ev, err := ParseEvent([]byte(`{"kind":"color","letter":"` + letter + `"}`))
	if err != nil {
		panic(err)
	}
	return ev
}

var (
	submit  = Event{Kind: KindSubmit}
	undo    = Event{Kind: KindUndo}
	help    = Event{Kind: KindHelp}
	closeEv = Event{Kind: KindClose}
	quit    = Event{Kind: KindQuit}
	choose0 = Event{Kind: KindChoose}
)

func name(s string) Event { return Event{Kind: KindText, Text: s} }

// guess sends the colors of c one by one, then Submit, reading the View
// shown after each tap. It returns the View after Submit.
func (h *harness) guess(c code.Code) View {
	h.t.Helper()
	for _, color := range c {
		h.send(colorEv(string(color.Abbrev())))
		h.expect(ScreenBoard)
	}
	h.send(submit)
	return h.next()
}

// toBoard plays the opening screens: the name, then Start on the line-up.
func (h *harness) toBoard(player string) View {
	h.t.Helper()
	h.expect(ScreenName)
	h.send(name(player))
	lineup := h.expect(ScreenMenu)
	h.send(choose0)
	_ = lineup
	return h.expect(ScreenBoard)
}

// TestSessionSingleGame plays a whole Single game, breaking the
// computer's code: the name, the line-up, a wrong guess, the right one,
// the result screen, and a second game from "Play again".
func TestSessionSingleGame(t *testing.T) {
	h := start(t)

	v := h.expect(ScreenName)
	if !v.Input.Text || v.Title == "" {
		t.Errorf("name screen = %+v, want a title and a text box", v)
	}
	h.send(name("  Tom  "))

	v = h.expect(ScreenMenu)
	want := []string{"Blue Tom vs Red Computer.", "Computer makes the code."}
	if strings.Join(v.Lines, "|") != strings.Join(want, "|") || len(v.Menu.Choices) != 1 {
		t.Errorf("line-up = %+v, want lines %q and one choice", v, want)
	}
	h.send(choose0)

	v = h.expect(ScreenBoard)
	if !v.Input.Palette || !v.Input.Undo || !v.Input.Submit || len(v.Board.Draft) != code.Length {
		t.Errorf("board = %+v, want the palette, Undo, Submit and an empty draft", v)
	}
	if v.Board.Title != "Single game · Round 1" || v.Lines[0] != "Row 1 of 12" {
		t.Errorf("board title %q, lines %q", v.Board.Title, v.Lines)
	}

	secret := secretFor()
	v = h.guess(wrong(secret))
	if v.Screen != ScreenBoard || len(v.Board.Rows) != 1 || v.Lines[0] != "Row 2 of 12" {
		t.Fatalf("after a wrong guess: %+v", v)
	}
	if v.Board.Draft[0] != (PegView{}) {
		t.Errorf("draft not cleared after a guess: %v", v.Board.Draft)
	}

	v = h.guess(secret)
	if v.Screen != ScreenResult {
		t.Fatalf("after the right guess: %+v", v)
	}
	want = []string{"Tom cracked the code in 2 rows!",
		"In a match that would score 12 points (solved in rows 1–4)."}
	if strings.Join(v.Lines, "|") != strings.Join(want, "|") {
		t.Errorf("result lines = %q, want %q", v.Lines, want)
	}
	if len(v.Board.Secret) != code.Length || v.Board.Secret[0].Letter != string(secret[0].Abbrev()) {
		t.Errorf("secret not revealed: %v", v.Board.Secret)
	}
	if v.Board.Draft != nil || v.Input.Palette || v.Menu == nil || v.Menu.Choices[0] != "Play again" {
		t.Errorf("result = %+v, want no draft or palette, and a Play again menu", v)
	}

	h.send(choose0)
	v = h.expect(ScreenMenu) // the line-up again, with the same name
	if v.Lines[0] != "Blue Tom vs Red Computer." {
		t.Errorf("second line-up = %q", v.Lines)
	}
}

// wrong returns a valid code that isn't secret.
func wrong(secret code.Code) code.Code {
	c, _ := code.Parse("RGYBK")
	if c == secret {
		c, _ = code.Parse("WPOGR")
	}
	return c
}

// TestSessionMistakes checks that mistakes on the board are explained and
// never cost a row: a color already in the row, Submit too soon, a full
// row, and a repeated guess. Undo removes a peg; the message clears on the
// next good tap.
func TestSessionMistakes(t *testing.T) {
	h := start(t)
	h.toBoard("Tom")

	h.send(colorEv("R"))
	h.expect(ScreenBoard)
	h.send(colorEv("r"))
	if v := h.expect(ScreenBoard); !strings.Contains(v.Message, "Red is used twice") {
		t.Errorf("duplicate color message = %q", v.Message)
	}
	h.send(submit)
	if v := h.expect(ScreenBoard); v.Message != "That's 1 color. A code needs exactly 5." {
		t.Errorf("early Submit message = %q", v.Message)
	}
	h.send(undo)
	if v := h.expect(ScreenBoard); v.Message != "" || v.Board.Draft[0] != (PegView{}) {
		t.Errorf("after Undo: message %q, draft %v", v.Message, v.Board.Draft)
	}
	h.send(undo) // nothing left to undo: shown again, no harm done
	h.expect(ScreenBoard)

	first := wrong(secretFor())
	h.guess(first)
	for _, c := range first {
		h.send(colorEv(string(c.Abbrev())))
		h.expect(ScreenBoard)
	}
	h.send(colorEv("O"))
	if v := h.expect(ScreenBoard); !strings.Contains(v.Message, "The row is full") {
		t.Errorf("full row message = %q", v.Message)
	}
	h.send(submit)
	v := h.expect(ScreenBoard)
	if !strings.Contains(v.Message, "You already tried") || len(v.Board.Rows) != 1 {
		t.Errorf("repeat guess: message %q, %d rows; want it refused", v.Message, len(v.Board.Rows))
	}
}

// TestSessionIgnoresStrayEvents checks that events meant for another
// screen do nothing: a color on the name screen, a name on the board.
func TestSessionIgnoresStrayEvents(t *testing.T) {
	h := start(t)
	h.expect(ScreenName)
	h.send(colorEv("R"), submit, choose0)
	h.quiet()
	h.send(name("Tom"))
	h.expect(ScreenMenu)
	h.send(name("Ann"), colorEv("R"))
	h.quiet()
	h.send(choose0)
	h.expect(ScreenBoard)
	h.send(name("Ann"), choose0, Event{Kind: KindReady}, Event{Kind: KindSkip})
	h.quiet()
}

// TestSessionNames checks the name rules: spaces are trimmed, an empty
// name becomes "Player", and a long one is cut to fit the screen.
func TestSessionNames(t *testing.T) {
	for typed, want := range map[string]string{
		"   ":                            "Player",
		"Ann":                            "Ann",
		"Bartholomew Montgomery-Smythe!": "Bartholomew Montgome",
		"Zoë":                            "Zoë",
	} {
		h := start(t)
		h.expect(ScreenName)
		h.send(name(typed))
		if v := h.expect(ScreenMenu); v.Lines[0] != "Blue "+want+" vs Red Computer." {
			t.Errorf("name %q: line-up %q, want the name %q", typed, v.Lines[0], want)
		}
	}
}

// TestSessionHelp checks that help opens over any screen and closing it
// shows that screen again, unchanged, including the draft.
func TestSessionHelp(t *testing.T) {
	h := start(t)
	h.expect(ScreenName)
	h.send(help)
	v := h.expect(ScreenHelp)
	if !strings.Contains(strings.Join(v.Lines, "\n"), "Rows 1–4: 12 points") {
		t.Errorf("help has no scoring table:\n%s", strings.Join(v.Lines, "\n"))
	}
	h.send(help) // already open: ignored
	h.quiet()
	h.send(closeEv)
	h.expect(ScreenName)
	h.send(closeEv) // not open: ignored
	h.quiet()

	h.send(name("Tom"), choose0)
	h.expect(ScreenMenu)
	h.expect(ScreenBoard)
	h.send(colorEv("G"))
	h.expect(ScreenBoard)
	h.send(help, colorEv("B")) // taps behind the help sheet are ignored
	h.expect(ScreenHelp)
	h.quiet()
	h.send(closeEv)
	if v := h.expect(ScreenBoard); v.Board.Draft[0].Letter != "G" || v.Board.Draft[1] != (PegView{}) {
		t.Errorf("after help, draft = %v, want just G", v.Board.Draft)
	}
}

// TestSessionQuit checks that quitting mid-game ends it without
// revealing the secret, and offers another game.
func TestSessionQuit(t *testing.T) {
	h := start(t)
	h.toBoard("Tom")
	h.send(quit)
	v := h.expect(ScreenResult)
	if v.Lines[0] != "Game stopped before the code was cracked." || v.Board.Secret != nil {
		t.Errorf("after quit: %+v", v)
	}
	h.send(choose0)
	h.expect(ScreenMenu)
}

// TestSessionOutOfRows plays 12 wrong guesses: the round ends unsolved
// and the secret is revealed.
func TestSessionOutOfRows(t *testing.T) {
	h := start(t)
	h.toBoard("Tom")
	secret := secretFor()
	var v View
	played := 0
	for _, c := range code.All() {
		if c == secret {
			continue
		}
		v = h.guess(c)
		if played++; played == 12 {
			break
		}
	}
	if v.Screen != ScreenResult || v.Lines[0] != "Tom didn't crack the code in 12 rows." ||
		v.Lines[1] != "No points this round." || len(v.Board.Secret) != code.Length {
		t.Errorf("after 12 wrong rows: %+v", v)
	}
}

// TestSessionStops checks that cancelling the context, as closing the page
// would, ends Run with the context's error, whichever screen is showing.
func TestSessionStops(t *testing.T) {
	for _, screens := range []int{0, 1, 2} {
		h := start(t)
		h.expect(ScreenName)
		if screens > 0 {
			h.send(name("Tom"))
			h.expect(ScreenMenu)
		}
		if screens > 1 {
			h.send(choose0)
			h.expect(ScreenBoard)
		}
		h.cancel()
		select {
		case err := <-h.done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("after %d screens: Run = %v, want context.Canceled", screens, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("after %d screens: Run didn't return after cancel", screens)
		}
	}
}

// TestHandleNeverBlocks checks the promise the browser depends on: Handle
// returns at once even when nothing is reading events, because a blocked
// JavaScript callback would freeze the page.
func TestHandleNeverBlocks(t *testing.T) {
	s := NewSession(newRNG(), func(View) {})
	done := make(chan struct{})
	go func() {
		for range 10000 {
			s.Handle(undo)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Handle blocked with no one reading")
	}
}

// TestResultViewJSON checks the result screen as the page receives it,
// as a golden file: the board with the secret revealed, the summary and
// the menu.
func TestResultViewJSON(t *testing.T) {
	h := start(t)
	h.toBoard("Tom")
	v := h.guess(secretFor())
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "result_single", got)
}

// TestTapperMakeSecret checks building a secret: the secret screen, with
// no board rows and the Breaker named; mistakes explained; Submit with a
// full row returns the code; Quit returns ErrQuit.
func TestTapperMakeSecret(t *testing.T) {
	views := make(chan View, 100)
	s := NewSession(newRNG(), func(v View) { views <- v })
	tp := &Tapper{s: s}
	info := game.RoundInfo{Number: 1, Mode: game.ModeSingle, MaxRows: game.MaxRows, Maker: tom, Breaker: computer}

	type result struct {
		c   code.Code
		err error
	}
	run := func() chan result {
		ch := make(chan result, 1)
		go func() {
			c, err := tp.MakeSecret(context.Background(), info)
			ch <- result{c, err}
		}()
		return ch
	}

	got := run()
	v := <-views
	if v.Screen != ScreenSecret || v.Lines[0] != "Computer will try to break it." ||
		len(v.Board.Rows) != 0 || len(v.Board.Draft) != code.Length {
		t.Fatalf("secret screen = %+v", v)
	}
	for _, l := range []string{"O", "O"} {
		s.Handle(colorEv(l))
		v = <-views
	}
	if !strings.Contains(v.Message, "Orange is used twice") {
		t.Errorf("duplicate message = %q", v.Message)
	}
	for _, l := range []string{"P", "Y", "G", "K"} {
		s.Handle(colorEv(l))
		<-views
	}
	s.Handle(submit)
	if r := <-got; r.err != nil || r.c != mustParse(t, "OPYGK") {
		t.Errorf("MakeSecret = %v, %v; want OPYGK", r.c, r.err)
	}

	got = run()
	<-views
	s.Handle(quit)
	if r := <-got; !errors.Is(r.err, ErrQuit) {
		t.Errorf("MakeSecret after Quit = %v, want ErrQuit", r.err)
	}
}

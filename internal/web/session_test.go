package web

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/tculpepp/codebreaker/internal/code"
	"github.com/tculpepp/codebreaker/internal/game"
	"github.com/tculpepp/codebreaker/internal/solver"
)

// Pacers for tests: instant lets the computer guess at once; never makes it
// wait until the player taps Skip. Neither ever sleeps.
var (
	instant Pacer = func() <-chan struct{} {
		c := make(chan struct{})
		close(c)
		return c
	}
	never Pacer = func() <-chan struct{} { return nil }
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

// secretFor predicts the computer's first secret when no coin is flipped:
// it's the first thing the session's generator is used for, so a twin
// generator gives the same code.
func secretFor() code.Code { return code.Random(newRNG()) }

// start runs a new session with the given pacer. It's stopped when the
// test ends.
func start(t *testing.T, pace Pacer) *harness {
	t.Helper()
	h := &harness{t: t, views: make(chan View, 1000), done: make(chan error, 1)}
	// Go note: the show function runs on the session's goroutine. Sending
	// each View on a buffered channel hands it to the test's goroutine
	// safely; the race detector would catch a shared variable instead.
	h.s = NewSession(newRNG(), pace, func(v View) { h.views <- v })
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

// until skips Views until one is on screen, and returns it.
func (h *harness) until(screen Screen) View {
	h.t.Helper()
	for {
		if v := h.next(); v.Screen == screen {
			return v
		}
	}
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

// colorEv is the event for tapping the color with this letter.
func colorEv(letter string) Event {
	ev, err := ParseEvent([]byte(`{"kind":"color","letter":"` + letter + `"}`))
	if err != nil {
		panic(err)
	}
	return ev
}

// Events, named so scripts read as what the player did.
var (
	submit  = Event{Kind: KindSubmit}
	undo    = Event{Kind: KindUndo}
	help    = Event{Kind: KindHelp}
	closeEv = Event{Kind: KindClose}
	quit    = Event{Kind: KindQuit}
	skip    = Event{Kind: KindSkip}
)

// choose is the event for tapping menu choice i (from 0).
func choose(i int) Event { return Event{Kind: KindChoose, Choice: i} }

// name is the event for entering a name.
func name(s string) Event { return Event{Kind: KindText, Text: s} }

// Menu answers, named like the terminal tests'.
var (
	menuMatch  = choose(0)
	menuSingle = choose(1)
	levelNorm  = choose(0)
	levelEasy  = choose(1)
	vsComputer = choose(0)
	vsHuman    = choose(1)
	roleBreak  = choose(0)
	roleMake   = choose(1)
	roleFlip   = choose(2)
	startGame  = choose(0)
	nextRound  = choose(0)
	playAgain  = choose(0)
	mainMenu   = choose(1)
)

// tap sends the colors of c one by one, reading the View shown after
// each.
func (h *harness) tap(c code.Code, screen Screen) {
	h.t.Helper()
	for _, color := range c {
		h.send(colorEv(string(color.Abbrev())))
		h.expect(screen)
	}
}

// guess taps the colors of c, then Submit, and returns the View after.
func (h *harness) guess(c code.Code) View {
	h.t.Helper()
	h.tap(c, ScreenBoard)
	h.send(submit)
	return h.next()
}

// menus answers the name and menu screens in order, checking each
// screen's title, and returns the line-up.
func (h *harness) menus(player string, answers ...Event) View {
	h.t.Helper()
	h.expect(ScreenName)
	h.send(name(player))
	for _, a := range answers {
		h.expect(ScreenMenu)
		h.send(a)
	}
	return h.expect(ScreenMenu)
}

// toBoard goes straight to breaking the computer's code in a Normal
// Single game.
func (h *harness) toBoard(player string) View {
	h.t.Helper()
	h.menus(player, menuSingle, levelNorm, vsComputer, roleBreak)
	h.send(startGame)
	return h.expect(ScreenBoard)
}

// joined returns a View's lines as one string, for checking.
func joined(v View) string { return strings.Join(v.Lines, "|") }

// TestSessionMenus checks each menu screen's title and choices, in order.
func TestSessionMenus(t *testing.T) {
	h := start(t, instant)
	h.expect(ScreenName)
	h.send(name("Tom"))
	want := []struct {
		title   string
		choices int
		answer  Event
	}{
		{"Main menu", 2, menuSingle},
		{"Difficulty", 2, levelNorm},
		{"Play against:", 2, vsComputer},
		{"Single game: who makes the code?", 3, roleBreak},
		{"Single game", 1, startGame},
	}
	for _, w := range want {
		v := h.expect(ScreenMenu)
		if v.Title != w.title || len(v.Menu.Choices) != w.choices {
			t.Errorf("menu %q with %d choices, want %q with %d", v.Title, len(v.Menu.Choices), w.title, w.choices)
		}
		h.send(choose(w.choices)) // out of range: ignored
		h.quiet()
		h.send(w.answer)
	}
	h.expect(ScreenBoard)
}

// TestSessionSingleBreaking plays a whole Single game, breaking the
// computer's code: the line-up, a wrong guess, the right one, the result
// screen, then Play again (same game) and Main menu.
func TestSessionSingleBreaking(t *testing.T) {
	h := start(t, instant)

	v := h.expect(ScreenName)
	if !v.Input.Text || v.Title == "" {
		t.Errorf("name screen = %+v, want a title and a text box", v)
	}
	h.send(name("  Tom  "))
	for _, a := range []Event{menuSingle, levelNorm, vsComputer, roleBreak} {
		h.expect(ScreenMenu)
		h.send(a)
	}
	v = h.expect(ScreenMenu)
	if got, want := joined(v), "Single game: Blue Tom vs Red Computer.|Computer makes the code."; got != want {
		t.Errorf("line-up = %q, want %q", got, want)
	}
	h.send(startGame)

	v = h.expect(ScreenBoard)
	if !v.Input.Palette || !v.Input.Undo || !v.Input.Submit || len(v.Board.Draft) != code.Length {
		t.Errorf("board = %+v, want the palette, Undo, Submit and an empty draft", v)
	}
	if v.Board.Title != "Single game · Round 1" || v.Lines[0] != "Row 1 of 12" || v.Board.Easy {
		t.Errorf("board title %q, lines %q, easy %v", v.Board.Title, v.Lines, v.Board.Easy)
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
	if v.Screen != ScreenResult || v.Title != "Game over" {
		t.Fatalf("after the right guess: %+v", v)
	}
	if got, want := joined(v), "Tom cracked the code in 2 rows!|In a match that would score 12 points (solved in rows 1–4)."; got != want {
		t.Errorf("result lines = %q, want %q", got, want)
	}
	if len(v.Board.Secret) != code.Length || v.Board.Secret[0].Letter != string(secret[0].Abbrev()) {
		t.Errorf("secret not revealed: %v", v.Board.Secret)
	}
	if v.Board.Draft != nil || v.Input.Palette || strings.Join(v.Menu.Choices, "|") != "Play again|Main menu" {
		t.Errorf("result = %+v, want no draft or palette, and Play again or Main menu", v)
	}

	h.send(playAgain)
	if v = h.expect(ScreenMenu); v.Title != "Single game" {
		t.Errorf("Play again showed %q, want the line-up", v.Title)
	}
	h.send(startGame)
	h.expect(ScreenBoard)
	h.send(quit)
	h.expect(ScreenResult)
	h.send(mainMenu)
	if v = h.expect(ScreenMenu); v.Title != "Main menu" {
		t.Errorf("Main menu showed %q", v.Title)
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
	h := start(t, instant)
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
	h.tap(first, ScreenBoard)
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
	h := start(t, instant)
	h.expect(ScreenName)
	h.send(colorEv("R"), submit, choose(0), skip)
	h.quiet()
	h.send(name("Tom"))
	h.expect(ScreenMenu)
	h.send(name("Ann"), colorEv("R"), skip)
	h.quiet()
	h.send(menuSingle, levelNorm, vsComputer, roleBreak, startGame)
	h.until(ScreenBoard)
	h.send(name("Ann"), choose(0), Event{Kind: KindReady}, skip)
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
		h := start(t, instant)
		v := h.menus(typed, menuSingle, levelNorm, vsComputer, roleBreak)
		if v.Lines[0] != "Single game: Blue "+want+" vs Red Computer." {
			t.Errorf("name %q: line-up %q, want the name %q", typed, v.Lines[0], want)
		}
	}
}

// TestSessionHelp checks that help opens over any screen and closing it
// shows that screen again, unchanged, including the draft.
func TestSessionHelp(t *testing.T) {
	h := start(t, instant)
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

	h.send(name("Tom"), menuSingle, levelNorm, vsComputer, roleBreak, startGame)
	h.until(ScreenBoard)
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

// TestSessionQuit checks that quitting mid-game ends it without revealing
// the secret, and offers another game.
func TestSessionQuit(t *testing.T) {
	h := start(t, instant)
	h.toBoard("Tom")
	h.send(quit)
	v := h.expect(ScreenResult)
	if v.Lines[0] != "Game stopped before the code was cracked." || v.Board.Secret != nil {
		t.Errorf("after quit: %+v", v)
	}
	h.send(playAgain)
	h.expect(ScreenMenu)
}

// TestSessionOutOfRows plays 12 wrong guesses: the round ends unsolved
// and the secret is revealed.
func TestSessionOutOfRows(t *testing.T) {
	h := start(t, instant)
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
	if v.Screen != ScreenResult || joined(v) != "Tom didn't crack the code in 12 rows.|No points this round." ||
		len(v.Board.Secret) != code.Length {
		t.Errorf("after 12 wrong rows: %+v", v)
	}
}

// TestSessionSingleMaking plays a Single game where the player makes the
// code and the computer breaks it: the secret screen, then the computer's
// rows one by one (paced), with the player's own code shown, and Skip
// offered; then the result.
func TestSessionSingleMaking(t *testing.T) {
	h := start(t, instant)
	v := h.menus("Tom", menuSingle, levelNorm, vsComputer, roleMake)
	if got, want := joined(v), "Single game: Blue Tom vs Red Computer.|Tom makes the code."; got != want {
		t.Errorf("line-up = %q, want %q", got, want)
	}
	h.send(startGame)
	v = h.expect(ScreenSecret)
	if v.Lines[0] != "Computer will try to break it." {
		t.Errorf("secret screen lines %q", v.Lines)
	}
	h.tap(mustParse(t, "OPYGK"), ScreenSecret)
	h.send(submit)

	v = h.expect(ScreenBoard)
	if !v.Input.Skip || v.Input.Palette || v.Board.Draft != nil || v.Lines[0] != "Computer is guessing: row 1 of 12" {
		t.Errorf("computer's turn = %+v, want Skip only and no draft", v)
	}
	if len(v.Board.Secret) != code.Length || v.Board.Secret[0].Letter != "O" {
		t.Errorf("player's own code not shown while the computer guesses: %v", v.Board.Secret)
	}

	v = h.until(ScreenResult)
	if !strings.HasPrefix(v.Lines[0], "Computer cracked the code in ") {
		t.Errorf("result lines %q", v.Lines)
	}
}

// TestSessionSkip checks Skip: with a pacer that would wait forever, the
// computer doesn't guess until the player taps Skip, and then plays the
// rest of the round at once. Quit also works while it waits.
func TestSessionSkip(t *testing.T) {
	h := start(t, never)
	h.menus("Tom", menuSingle, levelNorm, vsComputer, roleMake)
	h.send(startGame)
	h.expect(ScreenSecret)
	h.tap(mustParse(t, "OPYGK"), ScreenSecret)
	h.send(submit)
	h.expect(ScreenBoard)
	h.send(colorEv("R"), submit) // not for this screen
	h.quiet()
	h.send(skip)
	h.until(ScreenResult)

	h.send(playAgain, startGame)
	h.until(ScreenSecret)
	h.tap(mustParse(t, "OPYGK"), ScreenSecret)
	h.send(submit)
	h.expect(ScreenBoard)
	h.send(quit)
	if v := h.until(ScreenResult); v.Lines[0] != "Game stopped before the code was cracked." {
		t.Errorf("quit while the computer waits: %q", v.Lines)
	}
}

// TestSessionCoinFlip checks the third role choice: the line-up says the
// coin decided.
func TestSessionCoinFlip(t *testing.T) {
	h := start(t, instant)
	v := h.menus("Tom", menuSingle, levelNorm, vsComputer, roleFlip)
	if !strings.HasPrefix(v.Lines[1], "Coin flip: ") || !strings.HasSuffix(v.Lines[1], " makes the code.") {
		t.Errorf("line-up = %q, want the coin flip", v.Lines)
	}
}

// solverPlayer plays the human's side of a game well, using the real
// solver, so a test can play whole matches. It reads the feedback from
// each board View, just as a person reads the screen.
type solverPlayer struct {
	rng  *rand.Rand
	s    *solver.Solver
	seen int
}

// next returns a good guess for the board in v.
func (p *solverPlayer) next(t *testing.T, v View) code.Code {
	t.Helper()
	rows := v.Board.Rows
	if p.s == nil || len(rows) < p.seen {
		p.s, p.seen = solver.New(p.rng), 0
	}
	for _, r := range rows[p.seen:] {
		var g code.Code
		var fb code.Feedback
		for i, peg := range r.Guess {
			g[i], _ = code.ColorFromRune(rune(peg.Letter[0]))
		}
		for _, k := range r.Feedback {
			switch k {
			case "exact":
				fb.Exact++
			case "partial":
				fb.Partial++
			}
		}
		p.s.Update(g, fb)
	}
	p.seen = len(rows)
	c, err := p.s.Next()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestSessionMatch plays a whole Easy match against the computer: the
// line-up with the coin flip, rounds with swapping roles, a summary with
// the scores after each round but the last, and the winner at the end.
func TestSessionMatch(t *testing.T) {
	h := start(t, instant)
	v := h.menus("Tom", menuMatch, levelEasy, vsComputer)
	if v.Title != "Match" || v.Lines[0] != "Match: Blue Tom vs Red Computer. First to 30 points wins." ||
		!strings.HasPrefix(v.Lines[1], "Coin flip: ") {
		t.Errorf("line-up = %q %q", v.Title, v.Lines)
	}
	h.send(startGame)

	p := &solverPlayer{rng: rand.New(rand.NewPCG(9, 9))}
	var summaries, secrets int
	breakers := map[string]bool{}
	for {
		v := h.next()
		switch {
		case v.Screen == ScreenSecret:
			secrets++
			h.tap(mustParse(t, "OPYGK"), ScreenSecret)
			h.send(submit)
		case v.Screen == ScreenBoard && v.Input.Palette:
			if !v.Board.Easy || v.Board.Scores == nil {
				t.Fatalf("match board without Easy or scores: %+v", v.Board)
			}
			h.tap(p.next(t, v), ScreenBoard)
			h.send(submit)
		case v.Screen == ScreenSummary:
			summaries++
			breakers[v.Board.Breaker.Name] = true
			if !strings.HasPrefix(v.Title, "Round ") || !strings.Contains(joined(v), "Score: Blue Tom ") ||
				v.Menu.Choices[0] != "Next round" || len(v.Board.Secret) != code.Length {
				t.Errorf("summary = %+v", v)
			}
			h.send(nextRound)
		case v.Screen == ScreenResult:
			if v.Title != "Match over" || !strings.Contains(joined(v), "wins the match with ") ||
				!strings.Contains(joined(v), "Final score: Blue Tom ") {
				t.Errorf("match result = %q %q", v.Title, v.Lines)
			}
			if summaries < 2 || secrets < 1 || !breakers["Tom"] || !breakers[computerName] {
				t.Errorf("%d summaries, %d secrets, breakers %v: roles didn't swap", summaries, secrets, breakers)
			}
			return
		}
	}
}

// TestSessionMatchQuit checks quitting a match after a round: the
// standings so far are shown.
func TestSessionMatchQuit(t *testing.T) {
	h := start(t, instant)
	h.menus("Tom", menuMatch, levelNorm, vsComputer)
	h.send(startGame)
	p := &solverPlayer{rng: rand.New(rand.NewPCG(9, 9))}
	for {
		v := h.next()
		if v.Screen == ScreenSummary {
			break
		}
		if v.Screen == ScreenSecret {
			h.tap(mustParse(t, "OPYGK"), ScreenSecret)
			h.send(submit)
		}
		if v.Screen == ScreenBoard && v.Input.Palette {
			h.tap(p.next(t, v), ScreenBoard)
			h.send(submit)
		}
	}
	h.send(nextRound)
	for v := h.next(); v.Screen != ScreenSecret && !v.Input.Palette; v = h.next() {
	}
	h.send(quit)
	v := h.until(ScreenResult)
	if v.Title != "Match over" || !strings.HasPrefix(joined(v), "Match stopped after 1 round.|Score: Blue Tom ") {
		t.Errorf("quit match = %q %q", v.Title, v.Lines)
	}
}

// TestSessionStops checks that cancelling the context, as closing the page
// would, ends Run with the context's error, whichever screen is showing.
func TestSessionStops(t *testing.T) {
	for _, steps := range []int{0, 1, 2, 3} {
		h := start(t, never)
		h.expect(ScreenName)
		script := []Event{name("Tom"), menuSingle, levelNorm}
		for _, ev := range script[:steps] {
			h.send(ev)
			h.expect(ScreenMenu)
		}
		h.cancel()
		select {
		case err := <-h.done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("after %d steps: Run = %v, want context.Canceled", steps, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("after %d steps: Run didn't return after cancel", steps)
		}
	}
	for _, role := range []Event{roleBreak, roleMake} {
		h := start(t, never)
		h.menus("Tom", menuSingle, levelNorm, vsComputer, role)
		h.send(startGame)
		h.next()
		h.cancel()
		if err := <-h.done; !errors.Is(err, context.Canceled) {
			t.Errorf("during a game: Run = %v, want context.Canceled", err)
		}
	}
}

// TestSessionStopsMidGame checks closing the page at the other places a
// game waits: during the computer's pause, and on a match's round summary.
func TestSessionStopsMidGame(t *testing.T) {
	h := start(t, never)
	h.menus("Tom", menuSingle, levelNorm, vsComputer, roleMake)
	h.send(startGame)
	h.expect(ScreenSecret)
	h.tap(mustParse(t, "OPYGK"), ScreenSecret)
	h.send(submit)
	h.expect(ScreenBoard) // the computer is pausing before its first guess
	h.cancel()
	if err := <-h.done; !errors.Is(err, context.Canceled) {
		t.Errorf("during the pause: Run = %v, want context.Canceled", err)
	}

	h = start(t, instant)
	h.menus("Tom", menuMatch, levelNorm, vsComputer)
	h.send(startGame)
	p := &solverPlayer{rng: rand.New(rand.NewPCG(9, 9))}
	for v := h.next(); v.Screen != ScreenSummary; v = h.next() {
		if v.Screen == ScreenSecret {
			h.tap(mustParse(t, "OPYGK"), ScreenSecret)
			h.send(submit)
		}
		if v.Screen == ScreenBoard && v.Input.Palette {
			h.tap(p.next(t, v), ScreenBoard)
			h.send(submit)
		}
	}
	h.cancel()
	if err := <-h.done; !errors.Is(err, context.Canceled) {
		t.Errorf("on a round summary: Run = %v, want context.Canceled", err)
	}
}

// TestPlayBadSetup checks that a setup game.NewMatch refuses is passed up
// rather than ignored. The menus can't choose one, so the test sets an
// unknown mode directly.
func TestPlayBadSetup(t *testing.T) {
	s := NewSession(newRNG(), instant, func(View) {})
	s.name, s.mode = "Tom", game.Mode(9)
	if _, err := s.play(context.Background()); !errors.Is(err, game.ErrInvalidSetup) {
		t.Errorf("play with an unknown mode = %v, want game.ErrInvalidSetup", err)
	}
}

// TestDelay checks the real pacer: it fires after its delay, not before.
func TestDelay(t *testing.T) {
	start := time.Now()
	<-Delay(20 * time.Millisecond)()
	if d := time.Since(start); d < 20*time.Millisecond {
		t.Errorf("Delay fired after %v, want at least 20ms", d)
	}
}

// TestHandleNeverBlocks checks the promise the browser depends on: Handle
// returns at once even when nothing is reading events, because a blocked
// JavaScript callback would freeze the page.
func TestHandleNeverBlocks(t *testing.T) {
	s := NewSession(newRNG(), instant, func(View) {})
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
	h := start(t, instant)
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
	s := NewSession(newRNG(), instant, func(v View) { views <- v })
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
	// The page reads rows as a list on every board, so it must be an empty
	// list, never null. (A null here once stopped the game after the
	// Maker's first tap.)
	if data, _ := json.Marshal(v); !strings.Contains(string(data), `"rows":[]`) {
		t.Errorf("secret screen JSON has no empty rows list: %s", data)
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

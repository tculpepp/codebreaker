package web

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"

	"codebreaker/internal/code"
)

// ready is the event for tapping "I'm ready" on a handoff card.
var ready = Event{Kind: KindReady}

// toHumanGame goes from the name screen to the line-up of a Human vs
// Human game between Tom and Ann.
func (h *harness) toHumanGame(mode, role Event) View {
	h.t.Helper()
	h.expect(ScreenName)
	h.send(name("Tom"))
	for _, a := range []Event{mode, levelNorm, vsHuman} {
		h.expect(ScreenMenu)
		h.send(a)
	}
	h.expect(ScreenName)
	h.send(name("Ann"))
	if role.Kind != "" {
		h.expect(ScreenMenu)
		h.send(role)
	}
	return h.expect(ScreenMenu)
}

// noSecret fails the test if v reveals the secret: no secret row, and no
// board at all on a handoff card.
func noSecret(t *testing.T, v View) {
	t.Helper()
	if v.Board != nil && v.Board.Secret != nil {
		t.Errorf("%s screen shows the secret %v", v.Screen, v.Board.Secret)
	}
	if v.Screen == ScreenHandoff && v.Board != nil {
		t.Errorf("handoff card shows a board: %+v", v.Board)
	}
}

// TestSecondName checks the second player's name: the screen, a name
// already taken (ignoring case) refused with a message, and an empty name
// becoming "Player 2".
func TestSecondName(t *testing.T) {
	h := start(t, instant)
	h.expect(ScreenName)
	h.send(name("Tom"), menuSingle, levelNorm, vsHuman)
	v := h.until(ScreenName)
	if v.Title != "Second player" || !v.Input.Text {
		t.Errorf("second name screen = %+v", v)
	}
	h.send(name(" TOM "))
	if v = h.expect(ScreenName); v.Message != "Tom is already taken. Please choose another name." {
		t.Errorf("taken name message = %q", v.Message)
	}
	h.send(name(""))
	v = h.expect(ScreenMenu)
	if v.Title != "Single game: who makes the code?" || v.Menu.Choices[0] != "Tom breaks Player 2's code" ||
		v.Menu.Choices[1] != "Player 2 breaks Tom's code" {
		t.Errorf("role menu = %q %q", v.Title, v.Menu.Choices)
	}
}

// TestHumanVsHumanSingle plays a Single game where Tom breaks Ann's code:
// the device goes to Ann, she makes the code, it goes to Tom, and Tom
// never sees the code until the round ends.
func TestHumanVsHumanSingle(t *testing.T) {
	h := start(t, instant)
	v := h.toHumanGame(menuSingle, roleBreak)
	if got, want := joined(v), "Single game: Blue Tom vs Red Ann.|Ann makes the code."; got != want {
		t.Errorf("line-up = %q, want %q", got, want)
	}
	h.send(startGame)

	v = h.expect(ScreenHandoff)
	if v.Title != "Pass the device to Ann" || !v.Input.Ready || joined(v) != "Ann makes the code. Tom, no peeking!" {
		t.Errorf("first handoff = %+v", v)
	}
	noSecret(t, v)
	h.send(colorEv("R"), submit) // nothing to tap here but "I'm ready"
	h.quiet()
	h.send(ready)

	v = h.expect(ScreenSecret)
	if v.Lines[0] != "Tom will try to break it." {
		t.Errorf("secret screen lines %q", v.Lines)
	}
	secret := mustParse(t, "OPYGK")
	h.tap(secret, ScreenSecret)
	h.send(submit)

	// From here until the round ends, everything is on Tom's screen.
	v = h.expect(ScreenHandoff)
	if v.Title != "Pass the device to Tom" || joined(v) != "Tom breaks the code." {
		t.Errorf("second handoff = %+v", v)
	}
	noSecret(t, v)
	h.send(ready)
	v = h.expect(ScreenBoard)
	noSecret(t, v)
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "secret") {
		t.Errorf("Tom's board JSON mentions the secret: %s", data)
	}

	h.tap(wrong(secret), ScreenBoard)
	h.send(submit)
	noSecret(t, h.expect(ScreenBoard))
	v = h.guess(secret)
	if v.Screen != ScreenResult || v.Lines[0] != "Tom cracked the code in 2 rows!" || len(v.Board.Secret) != code.Length {
		t.Errorf("result = %+v", v)
	}
}

// TestHumanVsHumanMatch plays a whole match between two people, both
// played by the solver. Every View on a Breaker's turn must keep the
// secret hidden, and each round has two handoffs.
func TestHumanVsHumanMatch(t *testing.T) {
	h := start(t, instant)
	v := h.toHumanGame(menuMatch, Event{})
	if v.Lines[0] != "Match: Blue Tom vs Red Ann. First to 30 points wins." {
		t.Errorf("line-up = %q", v.Lines)
	}
	h.send(startGame)

	p := &solverPlayer{rng: rand.New(rand.NewPCG(9, 9))}
	handoffs, rounds := 0, 0
	for {
		v := h.next()
		switch v.Screen {
		case ScreenHandoff:
			handoffs++
			noSecret(t, v)
			h.send(ready)
		case ScreenSecret:
			h.tap(mustParse(t, "OPYGK"), ScreenSecret)
			h.send(submit)
		case ScreenBoard:
			noSecret(t, v)
			h.tap(p.next(t, v), ScreenBoard)
			h.send(submit)
		case ScreenSummary:
			rounds++
			h.send(nextRound)
		case ScreenResult:
			rounds++
			if !strings.Contains(joined(v), "wins the match") {
				t.Errorf("result = %q", v.Lines)
			}
			if handoffs != 2*rounds {
				t.Errorf("%d handoffs in %d rounds, want 2 per round", handoffs, rounds)
			}
			return
		}
	}
}

// TestHandoffQuit checks that Quit works on both handoff cards, without
// revealing the secret.
func TestHandoffQuit(t *testing.T) {
	h := start(t, instant)
	h.toHumanGame(menuSingle, roleBreak)
	h.send(startGame)
	h.expect(ScreenHandoff)
	h.send(quit)
	if v := h.expect(ScreenResult); v.Lines[0] != "Game stopped before the code was cracked." {
		t.Errorf("quit at handoff: %q", v.Lines)
	}
	h.send(playAgain, startGame)
	h.until(ScreenHandoff)
	h.send(ready)
	h.expect(ScreenSecret)
	h.tap(mustParse(t, "OPYGK"), ScreenSecret)
	h.send(submit)
	h.expect(ScreenHandoff) // the second card, before Tom's turn
	h.send(quit)
	if v := h.expect(ScreenResult); v.Lines[0] != "Game stopped before the code was cracked." || v.Board.Secret != nil {
		t.Errorf("quit at the second handoff: %+v", v)
	}

	h.send(mainMenu, menuSingle, levelNorm, vsHuman)
	h.until(ScreenName) // the second name is asked again
}

// TestHandoffViewJSON checks the handoff card as the page receives it.
func TestHandoffViewJSON(t *testing.T) {
	h := start(t, instant)
	h.toHumanGame(menuSingle, roleBreak)
	h.send(startGame)
	got, err := json.MarshalIndent(h.expect(ScreenHandoff), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "handoff", got)
}

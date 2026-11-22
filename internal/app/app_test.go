package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"codebreaker/internal/code"
	"codebreaker/internal/input"
	"codebreaker/internal/ui"
)

// update rewrites testdata/*.golden instead of comparing against them:
// `go test ./internal/app -update`, then review with `git diff`.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// checkGolden compares got with testdata/<name>.golden, or rewrites the
// file when -update is set.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/app -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- got ---\n%s--- want ---\n%s", path, got, want)
	}
}

// Scripted answers for the menus, named so the scripts below read as
// choices rather than bare numbers.
const (
	menuMatch  = "1" // main menu: Match against the computer
	menuSingle = "2" // main menu: Single game against the computer
	menuQuit   = "3" // main menu: Quit
	vsComputer = "1" // opponent menu: the computer
	vsHuman    = "2" // opponent menu: another person
	roleBreak  = "1" // role menu: break the computer's code
	roleMake   = "2" // role menu: make a code for the computer
	roleFlip   = "3" // role menu: flip a coin
	start      = ""  // "Press Enter to start"
	againNew   = "1" // play-again menu: New game
	againQuit  = "2" // play-again menu: Quit
)

// firstSecret returns the code the computer makes first, in a game where
// no coin is flipped. The app's generator is used first for that code, so
// a second generator with the same seed predicts it.
func firstSecret(seed uint64) code.Code {
	return code.Random(newRNG(seed))
}

// wrongGuess returns a valid code that isn't secret.
func wrongGuess(secret code.Code) string {
	if secret.String() == "RGYBK" {
		return "WPOGR"
	}
	return "RGYBK"
}

// script runs the app with scripted input and returns everything it
// wrote, transcript style (prompts followed by the "typed" answers).
func script(t *testing.T, ctx context.Context, seed uint64, lines ...string) (string, error) {
	t.Helper()
	var out strings.Builder
	err := run(ctx, input.NewScripted(&out, lines...), ui.NewRenderer(&out), newRNG(seed))
	return out.String(), err
}

// TestSingleTranscript plays a Single game and compares it with a saved
// transcript. Tom asks for help, picks a Single game breaking the
// computer's code, makes a typing mistake, a wrong guess and a repeat of
// it (neither mistake uses a row), solves on row 2, mistypes the
// play-again choice, goes back to the main menu and quits.
func TestSingleTranscript(t *testing.T) {
	s := firstSecret(1).String()
	wrong := wrongGuess(firstSecret(1))
	out, err := script(t, context.Background(), 1,
		"Tom", "?", menuSingle, vsComputer, roleBreak, start,
		"rrr", wrong, wrong, s, "5", againNew, menuQuit)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "single_transcript", out)
}

// seedComputerMakesFirst finds a seed whose coin flip makes the computer
// (seat 2) the first Code Maker. The coin flip is the first use of the
// app's generator in a match, so the test can check it in advance.
func seedComputerMakesFirst(t *testing.T) uint64 {
	t.Helper()
	for seed := range uint64(100) {
		if newRNG(seed).IntN(2) == 1 {
			return seed
		}
	}
	t.Fatal("no seed found")
	return 0
}

// TestMatchTranscript plays the first two rounds of a match and then
// quits, compared with a saved transcript. Round 1: the coin flip makes
// the computer the Maker and Tom solves its code. Round 2: roles swap, Tom
// makes a code and the computer breaks it. Round 3 starts with roles
// swapped back, and Tom quits, which shows the standings (D16).
func TestMatchTranscript(t *testing.T) {
	seed := seedComputerMakesFirst(t)
	rng := newRNG(seed)
	rng.IntN(2) // the coin flip
	secret := code.Random(rng)

	out, err := script(t, context.Background(), seed,
		"Tom", menuMatch, vsComputer, start, secret.String(), "opygb", "q")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Coin flip: Computer makes the first code.",
		"Tom scores",
		"Computer cracked the code",
		"Computer scores",
		"CODEBREAKER · Match · Round 3",
		"Match stopped after 2 rounds.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript is missing %q", want)
		}
	}
	checkGolden(t, "match_transcript", out)
}

// TestSingleAsMaker checks the other Single-game role: Tom makes the code
// and the computer breaks it.
func TestSingleAsMaker(t *testing.T) {
	out, err := script(t, context.Background(), 1,
		"Tom", menuSingle, vsComputer, roleMake, start, "rgybk", againQuit)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Tom makes the code.", "Maker: Blue Tom · Breaker: Red Computer", "Computer cracked the code"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// TestSingleCoinFlip checks that choosing the coin flip says who it
// picked. Either result is fine; quitting at the first prompt works for
// both roles.
func TestSingleCoinFlip(t *testing.T) {
	out, err := script(t, context.Background(), 1, "Tom", menuSingle, vsComputer, roleFlip, start, "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Coin flip: ") {
		t.Errorf("coin flip result not shown:\n%s", out)
	}
}

// TestHumanVsHumanSingle plays a Single game between two people and
// compares it with a saved transcript. Ann makes the code, typed hidden
// twice; the keyboard passes to Tom, who solves it on row 2. The code
// must not appear anywhere before Tom types it as a guess.
func TestHumanVsHumanSingle(t *testing.T) {
	out, err := script(t, context.Background(), 1,
		"Tom", menuSingle, vsHuman, "Ann", roleBreak, start,
		"opygb", "opygb", "", // Ann types the code twice, Tom presses Enter
		"rgybk", "opygb", againQuit)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Single game: Blue Tom vs Red Ann.",
		"Ann makes the code.",
		"Ann, type a secret code for Tom (hidden) ▸ (hidden)",
		"Pass the keyboard to Tom.",
		"Tom cracked the code in 2 rows!",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript is missing %q", want)
		}
	}
	firstGuess := strings.Index(out, "▸ opygb")
	if before := out[:firstGuess]; strings.Contains(strings.ToUpper(before), "OPYGB") {
		t.Errorf("secret visible before Tom guessed it")
	}
	checkGolden(t, "hvh_single_transcript", out)
}

// TestHumanVsHumanMatch plays a whole match between two people, to the
// end. Every round, the Maker types a code and the Breaker guesses it on
// row 1 for 12 points. Roles swap each round, so the first Breaker plays
// rounds 1, 3 and 5, reaches 36 in round 5 and wins at once, with the
// other player on 24 (D5: the match ends on the round that reaches 30).
func TestHumanVsHumanMatch(t *testing.T) {
	const seed = 1
	// The coin flip is the first use of the generator; seat 0 is Tom.
	firstMaker, firstBreaker := "Ann", "Tom"
	if newRNG(seed).IntN(2) == 0 {
		firstMaker, firstBreaker = "Tom", "Ann"
	}

	lines := []string{"Tom", menuMatch, vsHuman, "Ann", start}
	for _, secret := range []string{"rgybk", "wpogr", "kbyog", "gorwp", "ypkbw"} {
		lines = append(lines, secret, secret, "", secret)
	}
	lines = append(lines, againQuit)

	out, err := script(t, context.Background(), seed, lines...)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Coin flip: " + firstMaker + " makes the first code.",
		firstBreaker + " wins the match with 36 points after 5 rounds!",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "Pass the keyboard to"); got != 5 {
		t.Errorf("keyboard handed over %d times, want 5 (once per round)", got)
	}
}

// TestHumanVsHumanRoles checks the role menu names both people, and that
// the second choice makes the first player the Maker.
func TestHumanVsHumanRoles(t *testing.T) {
	out, err := script(t, context.Background(), 1, "Tom", menuSingle, vsHuman, "Ann", roleMake, start, "q")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Tom breaks Ann's code", "Ann breaks Tom's code", "Tom makes the code."} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// TestSecondName checks the second player's name: Enter gives "Player 2",
// help works, and a name already taken (in any case) is refused with a
// friendly message.
func TestSecondName(t *testing.T) {
	out, err := script(t, context.Background(), 1, "Tom", menuSingle, vsHuman, "?", "TOM", "", roleBreak, start, "q")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"How to play", "Tom is already playing. Please choose a different name.", "Red Player 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if err := (nameTakenError{name: "Tom"}); !strings.Contains(err.Error(), "Tom") {
		t.Errorf("nameTakenError.Error() = %q", err.Error())
	}
}

// TestDefaultName checks that pressing Enter at the name prompt plays as
// "Player".
func TestDefaultName(t *testing.T) {
	out, err := script(t, context.Background(), 1, "", menuSingle, vsComputer, roleBreak, start, "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Breaker: Blue Player") {
		t.Errorf("default name not used:\n%s", out)
	}
}

// TestHelpAtNamePrompt checks that help works at the very first prompt,
// and that the question is asked again afterwards.
func TestHelpAtNamePrompt(t *testing.T) {
	out, err := script(t, context.Background(), 1, "?", "Ann", menuSingle, vsComputer, roleBreak, start, "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "How to play") || strings.Count(out, "Your name") != 2 {
		t.Errorf("help not shown, or name not asked again:\n%s", out)
	}
	if !strings.Contains(out, "Breaker: Blue Ann") {
		t.Errorf("name after help not used:\n%s", out)
	}
}

// TestHelpAtStartPrompt checks that help works at "Press Enter to start",
// and that the question is asked again.
func TestHelpAtStartPrompt(t *testing.T) {
	out, err := script(t, context.Background(), 1, "Tom", menuSingle, vsComputer, roleBreak, "?", start, "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "How to play") || strings.Count(out, "Press Enter to start") != 2 {
		t.Errorf("help not shown, or start not asked again:\n%s", out)
	}
}

// TestQuitting checks every normal way to leave: q or the end of input at
// any prompt, or choosing Quit in a menu. Each one is a normal finish
// (nil error, exit code 0) that says goodbye (D16).
func TestQuitting(t *testing.T) {
	s := firstSecret(1).String()
	single := []string{"Tom", menuSingle, vsComputer, roleBreak, start}
	tests := []struct {
		name  string
		lines []string
	}{
		{name: "q at name", lines: []string{"q"}},
		{name: "input ends at name", lines: nil},
		{name: "quit from main menu", lines: []string{"Tom", menuQuit}},
		{name: "q at main menu", lines: []string{"Tom", "q"}},
		{name: "q at opponent menu", lines: []string{"Tom", menuSingle, "q"}},
		{name: "q at second name", lines: []string{"Tom", menuSingle, vsHuman, "q"}},
		{name: "q at role menu", lines: []string{"Tom", menuSingle, vsComputer, "q"}},
		{name: "q at start", lines: []string{"Tom", menuSingle, vsComputer, roleBreak, "q"}},
		{name: "q mid-round", lines: append(single, "q")},
		{name: "input ends mid-round", lines: single},
		{name: "quit from play-again", lines: append(single, s, againQuit)},
		{name: "q at play-again", lines: append(single, s, "q")},
		{name: "input ends at play-again", lines: append(single, s)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := script(t, context.Background(), 1, tt.lines...)
			if err != nil {
				t.Fatalf("run error = %v, want nil", err)
			}
			if !strings.HasSuffix(out, "Thanks for playing!\n") {
				t.Errorf("no goodbye at the end:\n%s", out)
			}
		})
	}
}

// TestMenuMistakesAndHelp checks that a bad menu answer is explained and
// asked again, and that help redraws the menu so the choices are visible.
func TestMenuMistakesAndHelp(t *testing.T) {
	out, err := script(t, context.Background(), 1, "Tom", "9", "?", menuQuit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Please type a number from 1 to 3.") {
		t.Errorf("bad choice not explained:\n%s", out)
	}
	if got := strings.Count(out, "Main menu"); got != 2 {
		t.Errorf("menu drawn %d times, want 2 (again after help)", got)
	}
}

// TestUnknownState checks the state machine's safety net: a state with no
// step function is reported as an error instead of looping forever.
func TestUnknownState(t *testing.T) {
	s := &session{}
	if _, err := s.step(context.Background(), state(99)); err == nil {
		t.Error("step(99) error = nil, want an error")
	}
}

// cancelOnCall is a Prompter that presses Ctrl-C (cancels the context)
// just before its nth question, then lets the real prompter answer.
type cancelOnCall struct {
	input.Prompter
	n, calls int
	cancel   context.CancelFunc
}

// Line counts the question and cancels on the chosen one.
//
// Go note: cancelOnCall embeds input.Prompter, so it gets the embedded
// value's Line method "for free". Declaring Line here overrides it, and
// p.Prompter.Line still reaches the original. This is a handy way to
// wrap an interface and change just one behavior.
func (p *cancelOnCall) Line(ctx context.Context, prompt string) (string, error) {
	p.calls++
	if p.calls == p.n {
		p.cancel()
	}
	return p.Prompter.Line(ctx, prompt)
}

// TestCtrlC checks that Ctrl-C during a round stops the game with a
// context error, which main turns into exit code 130, and that the
// summary is still shown.
func TestCtrlC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out strings.Builder
	// Questions: name, main menu, opponent, role, start, then the first
	// guess (6th).
	lines := input.NewScripted(&out, "Tom", menuSingle, vsComputer, roleBreak, start, "RGYBK")
	p := &cancelOnCall{Prompter: lines, n: 6, cancel: cancel}

	err := run(ctx, p, ui.NewRenderer(&out), newRNG(1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want context.Canceled", err)
	}
	if ExitCode(err) != ExitInterrupted {
		t.Errorf("ExitCode = %d, want %d", ExitCode(err), ExitInterrupted)
	}
	if !strings.Contains(out.String(), "Game stopped") {
		t.Errorf("no summary after Ctrl-C:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Thanks for playing") {
		t.Errorf("goodbye shown after Ctrl-C:\n%s", out.String())
	}
}

// TestRunReadError checks the public Run entry point end to end, with an
// input stream that fails: the failure is returned (exit code 1) with its
// cause still visible to errors.Is.
func TestRunReadError(t *testing.T) {
	errBoom := errors.New("boom")
	var out strings.Builder
	err := Run(context.Background(), iotest.ErrReader(errBoom), &out, 1)
	if !errors.Is(err, errBoom) {
		t.Fatalf("Run error = %v, want one wrapping %v", err, errBoom)
	}
	if ExitCode(err) != ExitError {
		t.Errorf("ExitCode = %d, want %d", ExitCode(err), ExitError)
	}
	if !strings.Contains(out.String(), "CODEBREAKER") {
		t.Errorf("Run didn't draw to out:\n%s", out.String())
	}
}

// TestExitCode checks the mapping from Run's error to the process exit
// code (implementation-plan.md §4, "Exit codes").
func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "finished", err: nil, want: ExitOK},
		{name: "ctrl-c", err: context.Canceled, want: ExitInterrupted},
		{name: "wrapped ctrl-c", err: fmt.Errorf("round 1: %w", context.Canceled), want: ExitInterrupted},
		{name: "anything else", err: errors.New("bug"), want: ExitError},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.err); got != tt.want {
			t.Errorf("%s: ExitCode = %d, want %d", tt.name, got, tt.want)
		}
	}
}

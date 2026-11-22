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

// secrets returns the first n codes the computer will pick with this seed.
// The app's generator is used only for the computer's codes in this
// version (the computer always makes the code, so there's no coin flip),
// so a second generator with the same seed predicts them.
func secrets(seed uint64, n int) []code.Code {
	rng := newRNG(seed)
	out := make([]code.Code, n)
	for i := range out {
		out[i] = code.Random(rng)
	}
	return out
}

// wrongGuess returns a valid code that isn't secret.
func wrongGuess(secret code.Code) string {
	if secret.String() == "RGYBK" {
		return "WPOGR"
	}
	return "RGYBK"
}

// session runs the app with scripted input and returns everything it
// wrote, transcript style (prompts followed by the "typed" answers).
func session(t *testing.T, ctx context.Context, seed uint64, lines ...string) (string, error) {
	t.Helper()
	var out strings.Builder
	err := run(ctx, input.NewScripted(&out, lines...), ui.NewRenderer(&out), newRNG(seed))
	return out.String(), err
}

// TestTranscript plays a whole session and compares it with a saved
// transcript. Tom asks for help, makes a typing mistake, a wrong guess and
// a repeat of it (neither mistake uses a row), solves the code on row 2,
// chooses to play again, then quits in the middle of the second game.
func TestTranscript(t *testing.T) {
	s := secrets(1, 1)
	wrong := wrongGuess(s[0])
	out, err := session(t, context.Background(), 1,
		"Tom", "?", "rrr", wrong, wrong, s[0].String(), "5", "1", "q")
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "transcript", out)
}

// TestDefaultName checks that pressing Enter at the name prompt plays as
// "Player".
func TestDefaultName(t *testing.T) {
	out, err := session(t, context.Background(), 1, "", "q")
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
	out, err := session(t, context.Background(), 1, "?", "Ann", "q")
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

// TestQuitting checks every normal way to leave: q or the end of input at
// any prompt, or choosing Quit in the menu. Each one is a normal finish
// (nil error, exit code 0) that says goodbye (D16).
func TestQuitting(t *testing.T) {
	s := secrets(1, 1)[0].String()
	tests := []struct {
		name  string
		lines []string
	}{
		{name: "q at name", lines: []string{"q"}},
		{name: "input ends at name", lines: nil},
		{name: "q mid-round", lines: []string{"Tom", "q"}},
		{name: "input ends mid-round", lines: []string{"Tom"}},
		{name: "quit from menu", lines: []string{"Tom", s, "2"}},
		{name: "q at menu", lines: []string{"Tom", s, "q"}},
		{name: "input ends at menu", lines: []string{"Tom", s}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := session(t, context.Background(), 1, tt.lines...)
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
	s := secrets(1, 1)[0].String()
	out, err := session(t, context.Background(), 1, "Tom", s, "9", "?", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Please type a number from 1 to 2.") {
		t.Errorf("bad choice not explained:\n%s", out)
	}
	if got := strings.Count(out, "Play again?"); got != 2 {
		t.Errorf("menu drawn %d times, want 2 (again after help)", got)
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
	p := &cancelOnCall{Prompter: input.NewScripted(&out, "Tom", "RGYBK"), n: 2, cancel: cancel}

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

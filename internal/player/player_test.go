package player

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
	"codebreaker/internal/input"
)

// seeded returns a random generator with a fixed seed, for reproducible
// tests.
func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

// mustParse turns a code string into a code.Code, failing the test on a
// typo in the test itself.
func mustParse(t *testing.T, s string) code.Code {
	t.Helper()
	c, err := code.Parse(s)
	if err != nil {
		t.Fatalf("bad code %q in test: %v", s, err)
	}
	return c
}

// fakeScreen records what a Human asked the screen to show, so tests can
// check which mistakes were reported and how often help was shown.
type fakeScreen struct {
	errs []error
	help int
}

// Go note: compile-time check that the fake really satisfies Screen.
var _ Screen = (*fakeScreen)(nil)

// Error records err.
func (s *fakeScreen) Error(err error) { s.errs = append(s.errs, err) }

// Help counts the call.
func (s *fakeScreen) Help() { s.help++ }

// info is a round where Tom breaks the computer's code.
var info = game.RoundInfo{
	Number: 1, Mode: game.ModeSingle, MaxRows: game.MaxRows,
	Maker:   game.Seat{Name: "Computer", Peg: game.PegRed},
	Breaker: game.Seat{Name: "Tom", Peg: game.PegBlue},
}

// TestHumanGuessRetriesUntilValid checks that each kind of mistake is
// reported once, in the order typed, and that the Human keeps asking
// until it gets a valid new guess (D14, implementation-plan.md §7).
func TestHumanGuessRetriesUntilValid(t *testing.T) {
	history := []game.Turn{{Row: 1, Guess: mustParse(t, "RGYBK")}}
	in := input.NewScripted(io.Discard,
		"RGXBK", // unknown letter
		"RGYB",  // too short
		"RGYRK", // duplicate color
		"rgybk", // repeat of row 1, in lower case
		"w p o g r",
	)
	screen := &fakeScreen{}
	h := NewHuman(in, screen)

	got, err := h.Guess(context.Background(), info, history)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustParse(t, "WPOGR"); got != want {
		t.Errorf("Guess = %v, want %v", got, want)
	}

	if len(screen.errs) != 4 {
		t.Fatalf("reported %d errors, want 4: %v", len(screen.errs), screen.errs)
	}
	var (
		invalid *code.InvalidColorError
		length  *code.LengthError
		dup     *code.DuplicateColorError
		repeat  *game.RepeatGuessError
	)
	if !errors.As(screen.errs[0], &invalid) {
		t.Errorf("error 1 = %v, want *code.InvalidColorError", screen.errs[0])
	}
	if !errors.As(screen.errs[1], &length) {
		t.Errorf("error 2 = %v, want *code.LengthError", screen.errs[1])
	}
	if !errors.As(screen.errs[2], &dup) {
		t.Errorf("error 3 = %v, want *code.DuplicateColorError", screen.errs[2])
	}
	if !errors.As(screen.errs[3], &repeat) || repeat.Row != 1 {
		t.Errorf("error 4 = %v, want *game.RepeatGuessError for row 1", screen.errs[3])
	}
}

// TestHumanGuessPrompt checks that the prompt shows which row is next out
// of how many, and reminds the player of the commands.
func TestHumanGuessPrompt(t *testing.T) {
	var out strings.Builder
	history := []game.Turn{
		{Row: 1, Guess: mustParse(t, "RGYBK")},
		{Row: 2, Guess: mustParse(t, "WPOGR")},
	}
	h := NewHuman(input.NewScripted(&out, "KWRBG"), &fakeScreen{})
	if _, err := h.Guess(context.Background(), info, history); err != nil {
		t.Fatal(err)
	}
	if want := " Row 3/12 (? help, q quit) ▸ KWRBG\n"; out.String() != want {
		t.Errorf("prompt = %q, want %q", out.String(), want)
	}
}

// TestHumanHelp checks that ? shows help and then asks the same question
// again, without counting as a mistake.
func TestHumanHelp(t *testing.T) {
	screen := &fakeScreen{}
	h := NewHuman(input.NewScripted(io.Discard, "?", "RGYBK"), screen)
	if _, err := h.Guess(context.Background(), info, nil); err != nil {
		t.Fatal(err)
	}
	if screen.help != 1 || len(screen.errs) != 0 {
		t.Errorf("help shown %d times with %d errors, want 1 and 0", screen.help, len(screen.errs))
	}
}

// TestHumanStops checks that quitting, the end of input and Ctrl-C all
// stop the Human at once and are passed up unchanged, so the match can
// end and show the standings (D16).
func TestHumanStops(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name  string
		ctx   context.Context
		lines []string
		want  error
	}{
		{name: "quit", ctx: context.Background(), lines: []string{"q"}, want: input.ErrQuit},
		{name: "end of input", ctx: context.Background(), want: io.EOF},
		{name: "ctrl-c", ctx: cancelled, lines: []string{"RGYBK"}, want: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHuman(input.NewScripted(io.Discard, tt.lines...), &fakeScreen{})
			if _, err := h.Guess(tt.ctx, info, nil); !errors.Is(err, tt.want) {
				t.Errorf("Guess error = %v, want %v", err, tt.want)
			}
			if _, err := NewHuman(input.NewScripted(io.Discard, tt.lines...), &fakeScreen{}).
				MakeSecret(tt.ctx, info); !errors.Is(err, tt.want) {
				t.Errorf("MakeSecret error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestHumanMakeSecret checks that a human Code Maker is asked for a valid
// code, with mistakes reported, and that the prompt names both players.
// (Hidden typing for Human vs Human comes in v0.5.0.)
func TestHumanMakeSecret(t *testing.T) {
	var out strings.Builder
	screen := &fakeScreen{}
	h := NewHuman(input.NewScripted(&out, "RRGYB", "rgybk"), screen)

	got, err := h.MakeSecret(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustParse(t, "RGYBK"); got != want {
		t.Errorf("MakeSecret = %v, want %v", got, want)
	}
	if len(screen.errs) != 1 {
		t.Errorf("reported %d errors, want 1", len(screen.errs))
	}
	if !strings.Contains(out.String(), "Computer, choose a secret code for Tom") {
		t.Errorf("prompt doesn't name the players:\n%s", out.String())
	}
}

// TestComputerMakeSecret checks that the computer's secrets are valid and
// depend only on the seed, so a seeded game is reproducible.
func TestComputerMakeSecret(t *testing.T) {
	a, b := NewComputer(seeded(7)), NewComputer(seeded(7))
	for i := range 20 {
		sa, err := a.MakeSecret(context.Background(), info)
		if err != nil {
			t.Fatal(err)
		}
		sb, _ := b.MakeSecret(context.Background(), info)
		if err := sa.Validate(); err != nil {
			t.Errorf("secret %d (%v) is invalid: %v", i, sa, err)
		}
		if sa != sb {
			t.Errorf("secret %d: %v and %v differ with the same seed", i, sa, sb)
		}
	}
}

// TestComputerCannotGuessYet records a known gap: the computer learns to
// break codes in v0.4.0. Until then Guess fails clearly instead of
// guessing nonsense.
func TestComputerCannotGuessYet(t *testing.T) {
	_, err := NewComputer(seeded(1)).Guess(context.Background(), info, nil)
	if !errors.Is(err, ErrCannotBreak) {
		t.Errorf("Guess error = %v, want ErrCannotBreak", err)
	}
}

// TestHumanBreaksComputerInAMatch plays a real Single game: the computer
// makes the code, and the human types a bad guess, a wrong guess, the same
// wrong guess again, and then the answer. Only the two valid guesses use a
// row, so the round is solved in 2 rows for 12 points.
func TestHumanBreaksComputerInAMatch(t *testing.T) {
	// The computer's secret depends only on the seed, so a second
	// generator with the same seed shows the test what it will be.
	secret := code.Random(seeded(3))
	wrong := mustParse(t, "RGYBK")
	if wrong == secret {
		wrong = mustParse(t, "WPOGR")
	}

	in := input.NewScripted(io.Discard, "nope", wrong.String(), wrong.String(), secret.String())
	seats := [2]game.Seat{
		{Name: "Tom", Peg: game.PegBlue, Player: NewHuman(in, &fakeScreen{})},
		{Name: "Computer", Peg: game.PegRed, Player: NewComputer(seeded(3))},
	}
	m, err := game.NewMatch(game.Config{Mode: game.ModeSingle, FirstMaker: game.FirstMakerSeat1}, seats, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.PlayRound(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved || res.Rows != 2 || res.Points != 12 {
		t.Errorf("result = solved %v in %d rows for %d points, want solved in 2 rows for 12",
			res.Solved, res.Rows, res.Points)
	}
}

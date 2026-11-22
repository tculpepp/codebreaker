package player

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
	"codebreaker/internal/input"
	"codebreaker/internal/solver"
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
// check which mistakes were reported, how often help was shown, and when
// the keyboard was handed over.
type fakeScreen struct {
	errs     []error
	help     int
	handoffs []string // who the keyboard was passed to, in order
	redraws  int
}

// Go note: compile-time check that the fake really satisfies Screen.
var _ Screen = (*fakeScreen)(nil)

// Error records err.
func (s *fakeScreen) Error(err error) { s.errs = append(s.errs, err) }

// Help counts the call.
func (s *fakeScreen) Help() { s.help++ }

// Handoff records who the keyboard goes to.
func (s *fakeScreen) Handoff(next string) { s.handoffs = append(s.handoffs, next) }

// Redraw counts the call.
func (s *fakeScreen) Redraw() { s.redraws++ }

// Prompt formats a prompt the way the Unicode renderer does.
func (s *fakeScreen) Prompt(text string) string { return " " + text + " ▸ " }

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

// hvhInfo is a Human vs Human round where Tom makes the code for Ann.
var hvhInfo = game.RoundInfo{
	Number: 1, Mode: game.ModeMatch, MaxRows: game.MaxRows,
	Maker:   game.Seat{Name: "Tom", Peg: game.PegBlue},
	Breaker: game.Seat{Name: "Ann", Peg: game.PegRed},
}

// hiddenHuman returns a Human that types secrets hidden, answering with
// lines, and the screen and transcript it uses.
func hiddenHuman(lines ...string) (*Human, *fakeScreen, *strings.Builder) {
	var out strings.Builder
	screen := &fakeScreen{}
	h := NewHuman(input.NewScripted(&out, lines...), screen)
	h.Hidden = true
	return h, screen, &out
}

// TestHiddenSecret checks the Human vs Human Code Maker (D10): the code is
// typed hidden twice, then the keyboard is handed to the Breaker, who
// presses Enter and sees the board again. The code never appears in the
// output.
func TestHiddenSecret(t *testing.T) {
	h, screen, out := hiddenHuman("rgybk", "RGYBK", "")
	got, err := h.MakeSecret(context.Background(), hvhInfo)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustParse(t, "RGYBK"); got != want {
		t.Errorf("MakeSecret = %v, want %v", got, want)
	}
	if !slices.Equal(screen.handoffs, []string{"Ann"}) || screen.redraws != 1 {
		t.Errorf("handoffs %v, redraws %d; want [Ann] and 1", screen.handoffs, screen.redraws)
	}
	if strings.Contains(strings.ToUpper(out.String()), "RGYBK") {
		t.Errorf("secret visible in output:\n%s", out.String())
	}
	for _, want := range []string{"Tom, type a secret code for Ann", "Type it again", "Ann, press Enter"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}

// TestHiddenSecretMistakes checks that a mistyped code and a second entry
// that doesn't match are both reported, and the Maker starts again
// without the keyboard being handed over early.
func TestHiddenSecretMistakes(t *testing.T) {
	h, screen, _ := hiddenHuman(
		"rgyrk",          // duplicate color: asked again
		"rgybk", "wpogr", // don't match: start again
		"rgybk", "rgybk", "")
	got, err := h.MakeSecret(context.Background(), hvhInfo)
	if err != nil || got != mustParse(t, "RGYBK") {
		t.Fatalf("MakeSecret = %v, %v; want RGYBK, nil", got, err)
	}
	if len(screen.errs) != 2 {
		t.Fatalf("reported %d errors, want 2: %v", len(screen.errs), screen.errs)
	}
	var dup *code.DuplicateColorError
	if !errors.As(screen.errs[0], &dup) {
		t.Errorf("error 1 = %v, want *code.DuplicateColorError", screen.errs[0])
	}
	if !errors.Is(screen.errs[1], ErrSecretMismatch) {
		t.Errorf("error 2 = %v, want ErrSecretMismatch", screen.errs[1])
	}
	if len(screen.handoffs) != 1 {
		t.Errorf("handed off %d times, want 1", len(screen.handoffs))
	}
}

// TestSecretMismatchIsFriendly checks the message the screen shows for a
// mismatch, through the Friendly method that ui looks for.
func TestSecretMismatchIsFriendly(t *testing.T) {
	f, ok := ErrSecretMismatch.(interface{ Friendly() string })
	if !ok || !strings.Contains(f.Friendly(), "don't match") {
		t.Errorf("ErrSecretMismatch has no suitable Friendly message")
	}
	if !strings.Contains(ErrSecretMismatch.Error(), "differ") {
		t.Errorf("ErrSecretMismatch.Error() = %q", ErrSecretMismatch.Error())
	}
}

// TestHiddenSecretHelpAndQuit checks help and quit at each of the three
// prompts: first entry, second entry, and the Breaker's "press Enter".
// Help asks again; quit stops at once.
func TestHiddenSecretHelpAndQuit(t *testing.T) {
	h, screen, _ := hiddenHuman("?", "rgybk", "?", "rgybk", "?", "")
	if _, err := h.MakeSecret(context.Background(), hvhInfo); err != nil {
		t.Fatal(err)
	}
	if screen.help != 3 {
		t.Errorf("help shown %d times, want 3", screen.help)
	}

	for _, lines := range [][]string{{"q"}, {"rgybk", "q"}, {"rgybk", "rgybk", "q"}} {
		h, _, _ := hiddenHuman(lines...)
		if _, err := h.MakeSecret(context.Background(), hvhInfo); !errors.Is(err, input.ErrQuit) {
			t.Errorf("lines %q: MakeSecret error = %v, want ErrQuit", lines, err)
		}
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

// TestComputerGuessFitsFeedback checks the computer as Code Breaker: its
// first guess is the solver's opening, and every later guess could still
// be the secret given all the feedback in history. It only ever sees
// history, never the secret.
func TestComputerGuessFitsFeedback(t *testing.T) {
	secret := mustParse(t, "OPYGB")
	c := NewComputer(seeded(5))
	var history []game.Turn
	for row := 1; row <= game.MaxRows; row++ {
		g, err := c.Guess(context.Background(), info, history)
		if err != nil {
			t.Fatal(err)
		}
		for _, past := range history {
			if code.Score(g, past.Guess) != past.Feedback {
				t.Fatalf("row %d guess %v contradicts row %d", row, g, past.Row)
			}
		}
		fb := code.Score(secret, g)
		history = append(history, game.Turn{Row: row, Guess: g, Feedback: fb})
		if fb.Solved() {
			return
		}
	}
	t.Fatalf("not solved in %d rows: %v", game.MaxRows, history)
}

// TestComputerStartsFreshEachRound checks that the computer forgets the
// last round when a new one starts (an empty history), and that it rebuilds
// its knowledge if handed a history it hasn't followed. Either way its
// guesses must fit the history it was given.
func TestComputerStartsFreshEachRound(t *testing.T) {
	c := NewComputer(seeded(9))
	first, _ := c.Guess(context.Background(), info, nil)

	// Round 1: one guess with 3 right colors, all in the wrong spots.
	// (A guess can't share fewer than 2 colors with the secret: 5 + 5
	// colors out of 8 must overlap by at least 2.)
	turn := game.Turn{Row: 1, Guess: first, Feedback: code.Feedback{Partial: 3}}
	if _, err := c.Guess(context.Background(), info, []game.Turn{turn}); err != nil {
		t.Fatal(err)
	}

	// Round 2: an empty history means a new round, so the opening again.
	again, _ := c.Guess(context.Background(), info, nil)
	if again != first {
		t.Errorf("new round opened with %v, want %v", again, first)
	}

	// A different history than the one followed so far: the guess must
	// fit it, not the old feedback.
	other := game.Turn{Row: 1, Guess: first, Feedback: code.Feedback{Exact: 2, Partial: 2}}
	fresh := NewComputer(seeded(9))
	_, _ = fresh.Guess(context.Background(), info, []game.Turn{turn, turn}) // follows 2 rows
	g, err := fresh.Guess(context.Background(), info, []game.Turn{other})   // then only 1
	if err != nil {
		t.Fatal(err)
	}
	if code.Score(g, first) != other.Feedback {
		t.Errorf("guess %v doesn't fit the history it was given", g)
	}
}

// TestComputerGuessReportsImpossibleFeedback checks that feedback no code
// could produce comes back as the solver's error rather than a nonsense
// guess. The game computes feedback itself, so this only guards against
// bugs.
func TestComputerGuessReportsImpossibleFeedback(t *testing.T) {
	c := NewComputer(seeded(1))
	bad := game.Turn{Row: 1, Guess: mustParse(t, "RGYBK"), Feedback: code.Feedback{Exact: 4, Partial: 1}}
	if _, err := c.Guess(context.Background(), info, []game.Turn{bad}); !errors.Is(err, solver.ErrNoCandidates) {
		t.Errorf("Guess error = %v, want solver.ErrNoCandidates", err)
	}
}

// TestComputerVsComputer plays full matches between two computers, which
// the menu doesn't offer but tests and demos use (D17). Every match must
// end with exactly one player at 30 or more, every round must be solved
// within 12 rows, and the same seed must replay the same match.
func TestComputerVsComputer(t *testing.T) {
	play := func(seed uint64) game.Standings {
		rng := seeded(seed)
		seats := [2]game.Seat{
			{Name: "Deep", Peg: game.PegBlue, Player: NewComputer(rng)},
			{Name: "Blue", Peg: game.PegRed, Player: NewComputer(rng)},
		}
		m, err := game.NewMatch(game.Config{}, seats, rng, nil)
		if err != nil {
			t.Fatal(err)
		}
		st, err := m.Play(context.Background())
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		return st
	}

	for seed := range uint64(30) {
		st := play(seed)
		if st.Winner < 0 {
			t.Fatalf("seed %d: no winner: %+v", seed, st)
		}
		winner, loser := st.Seats[st.Winner], st.Seats[1-st.Winner]
		if winner.Score < game.TargetScore || loser.Score >= game.TargetScore {
			t.Errorf("seed %d: scores %d and %d, want exactly one at %d or more",
				seed, winner.Score, loser.Score, game.TargetScore)
		}
		for _, r := range st.Results {
			if !r.Solved {
				t.Errorf("seed %d: round %d not solved in %d rows", seed, r.Number, r.Rows)
			}
		}
	}

	// Seats hold different Player pointers in each run, so compare the
	// scores and the round results instead.
	a, b := play(99), play(99)
	if a.Seats[0].Score != b.Seats[0].Score || a.Seats[1].Score != b.Seats[1].Score ||
		!slices.Equal(a.Results, b.Results) {
		t.Errorf("same seed, different matches:\n%+v\n%+v", a, b)
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

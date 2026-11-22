package game

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
)

// seeded returns a random generator with a fixed seed, for reproducible
// tests.
func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

// TestNewMatchRejectsBadSetup checks that each unusable setup is refused
// with an error wrapping ErrInvalidSetup, rather than failing later in play.
func TestNewMatchRejectsBadSetup(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		edit  func(*[2]Seat) // changes the seats to make them invalid
		noRNG bool
	}{
		{name: "missing player", edit: func(s *[2]Seat) { s[1].Player = nil }},
		{name: "empty name", edit: func(s *[2]Seat) { s[0].Name = "" }},
		{name: "same peg color", edit: func(s *[2]Seat) { s[1].Peg = PegBlue }},
		{name: "unknown peg color", edit: func(s *[2]Seat) { s[1].Peg = PegColor(7) }},
		{name: "unknown mode", cfg: Config{Mode: Mode(9)}},
		{name: "unknown first maker", cfg: Config{FirstMaker: FirstMaker(9)}},
		{name: "coin flip needs a random generator", noRNG: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, seats := newPair()
			if tt.edit != nil {
				tt.edit(&seats)
			}
			rng := seeded(1)
			if tt.noRNG {
				rng = nil
			}
			_, err := NewMatch(tt.cfg, seats, rng, nil)
			if !errors.Is(err, ErrInvalidSetup) {
				t.Errorf("NewMatch error = %v, want one wrapping ErrInvalidSetup", err)
			}
		})
	}
}

// TestNewMatchStartsScoresAtZero checks that scores passed in are ignored:
// every match starts from 0-0.
func TestNewMatchStartsScoresAtZero(t *testing.T) {
	_, _, seats := newPair()
	seats[0].Score = 25
	m, err := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Seats(); got[0].Score != 0 || got[1].Score != 0 {
		t.Errorf("starting scores = %d-%d, want 0-0", got[0].Score, got[1].Score)
	}
}

// TestCoinFlip checks D6: with FirstMakerCoinFlip, the first Code Maker is
// random but reproducible. The same seed always picks the same seat, and
// across seeds both seats get picked.
func TestCoinFlip(t *testing.T) {
	_, _, seats := newPair()
	picked := map[int]bool{}
	for seed := range uint64(50) {
		m1, _ := NewMatch(Config{}, seats, seeded(seed), nil)
		m2, _ := NewMatch(Config{}, seats, seeded(seed), nil)
		if m1.MakerIdx() != m2.MakerIdx() {
			t.Fatalf("seed %d: same seed picked different first makers", seed)
		}
		picked[m1.MakerIdx()] = true
	}
	if !picked[0] || !picked[1] {
		t.Errorf("over 50 seeds the coin flip only picked %v", picked)
	}
}

// TestExplicitFirstMaker checks that choosing the first maker (as Single
// Game's role menu does, D13) works without a random generator.
func TestExplicitFirstMaker(t *testing.T) {
	_, _, seats := newPair()
	for _, tt := range []struct {
		fm   FirstMaker
		want int
	}{{FirstMakerSeat0, 0}, {FirstMakerSeat1, 1}} {
		m, err := NewMatch(Config{FirstMaker: tt.fm}, seats, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if m.MakerIdx() != tt.want {
			t.Errorf("FirstMaker %v: MakerIdx = %d, want %d", tt.fm, m.MakerIdx(), tt.want)
		}
	}
}

// TestSingleGame checks D13: Single mode plays exactly one round, reports
// the points the bracket gives, and has no match winner.
func TestSingleGame(t *testing.T) {
	_, bob, seats := newPair()
	bob.solveAt[1] = 5 // Bob breaks Alice's code on row 5 → 10 points
	rec := &recorder{}
	m, _ := NewMatch(Config{Mode: ModeSingle, FirstMaker: FirstMakerSeat0}, seats, nil, rec)

	st, err := m.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Results) != 1 {
		t.Fatalf("played %d rounds, want 1", len(st.Results))
	}
	res := st.Results[0]
	if res.MakerIdx != 0 || res.BreakerIdx != 1 || !res.Solved || res.Rows != 5 || res.Points != 10 {
		t.Errorf("result = %+v, want Alice making, Bob solving in 5 rows for 10 points", res)
	}
	if res.Secret != mustParse(t, "RGYBK") {
		t.Errorf("result secret = %v, want RGYBK", res.Secret)
	}
	if st.Winner != -1 {
		t.Errorf("Winner = %d, want -1 (no match winner in Single mode)", st.Winner)
	}
	if !m.Over() {
		t.Error("Over() = false after a Single game")
	}
	if _, err := m.PlayRound(context.Background()); !errors.Is(err, ErrMatchOver) {
		t.Errorf("PlayRound after end = %v, want ErrMatchOver", err)
	}
}

// TestObserverEvents checks the exact order of events the screen receives
// for a short Single game.
func TestObserverEvents(t *testing.T) {
	_, bob, seats := newPair()
	bob.solveAt[1] = 2
	rec := &recorder{}
	m, _ := NewMatch(Config{Mode: ModeSingle, FirstMaker: FirstMakerSeat0}, seats, nil, rec)
	m.Play(context.Background())

	want := []string{
		"start r1 maker=Alice breaker=Bob",
		"turn r1 row1",
		"turn r1 row2",
		"end r1 solved=true rows=2 pts=12 score=0-12",
		"match winner=-1",
	}
	if !slices.Equal(rec.events, want) {
		t.Errorf("events:\n%s\nwant:\n%v", rec, want)
	}
}

// TestMatchRolesAlternate checks D6: the maker and breaker swap every round.
func TestMatchRolesAlternate(t *testing.T) {
	alice, bob, seats := newPair()
	for r := 1; r <= 6; r++ {
		alice.solveAt[r] = 1
		bob.solveAt[r] = 1
	}
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat1}, seats, nil, nil)
	st, err := m.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, res := range st.Results {
		wantMaker := 1 - i%2 // seat 1 first, then 0, 1, 0, ...
		if res.MakerIdx != wantMaker || res.BreakerIdx != 1-wantMaker {
			t.Errorf("round %d: maker=%d breaker=%d, want maker=%d", res.Number, res.MakerIdx, res.BreakerIdx, wantMaker)
		}
	}
}

// TestMatchEndsWhenTargetReached checks D5 and D15: the match ends on the
// round a player reaches 30, even if the other player has scored faster
// and has had one fewer turn as breaker.
//
// Script: Bob makes first, so Alice breaks in rounds 1, 3, 5 and Bob in
// rounds 2, 4. Alice solves on row 5 each time (10 points); Bob solves on
// row 1 (12 points).
//
//	round 1: Alice 10   Bob  0
//	round 2: Alice 10   Bob 12
//	round 3: Alice 20   Bob 12
//	round 4: Alice 20   Bob 24
//	round 5: Alice 30 → Alice wins, exactly on target; round 6 never happens
func TestMatchEndsWhenTargetReached(t *testing.T) {
	alice, bob, seats := newPair()
	for _, r := range []int{1, 3, 5, 7} {
		alice.solveAt[r] = 5
	}
	for _, r := range []int{2, 4, 6} {
		bob.solveAt[r] = 1
	}
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat1}, seats, nil, nil)
	st, err := m.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Results) != 5 {
		t.Errorf("rounds played = %d, want 5", len(st.Results))
	}
	if st.Winner != 0 || st.Seats[0].Score != 30 || st.Seats[1].Score != 24 {
		t.Errorf("winner=%d score=%d-%d, want Alice (0) winning 30-24",
			st.Winner, st.Seats[0].Score, st.Seats[1].Score)
	}
}

// TestMatchOvershootsTarget checks that going past 30 still wins. Here
// both players solve on row 1 every time (12 points), so the first breaker
// reaches 36 in round 5.
func TestMatchOvershootsTarget(t *testing.T) {
	alice, bob, seats := newPair()
	for r := 1; r <= 9; r++ {
		alice.solveAt[r] = 1
		bob.solveAt[r] = 1
	}
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)
	st, _ := m.Play(context.Background())

	// Alice makes first, so Bob breaks in rounds 1, 3, 5.
	if st.Winner != 1 || st.Seats[1].Score != 36 || st.Seats[0].Score != 24 {
		t.Errorf("winner=%d score=%d-%d, want Bob (1) winning 24-36",
			st.Winner, st.Seats[0].Score, st.Seats[1].Score)
	}
}

// TestUnsolvedRoundScoresZero checks a round where the breaker runs out of
// rows: all 12 rows are used, nothing is scored, and play continues.
func TestUnsolvedRoundScoresZero(t *testing.T) {
	_, _, seats := newPair() // no solveAt entries: nobody ever solves
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)

	res, err := m.PlayRound(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Solved || res.Rows != MaxRows || res.Points != 0 {
		t.Errorf("result = %+v, want unsolved after %d rows with 0 points", res, MaxRows)
	}
	if m.Over() {
		t.Error("match ended after an unsolved round; it should continue")
	}
	if m.MakerIdx() != 1 {
		t.Errorf("next maker = %d, want 1 (roles swap even when unsolved)", m.MakerIdx())
	}
}

// errBoom is a stand-in for any error a player might return, such as the
// quit command or a closed input.
var errBoom = errors.New("boom")

// TestPlayerErrorEndsMatch checks that an error from either player stops
// the match, is passed back to the caller (still recognizable with
// errors.Is), and still reports the standings with no winner (D16: quit
// shows the standings).
func TestPlayerErrorEndsMatch(t *testing.T) {
	tests := []struct {
		name  string
		setup func(alice, bob *fakePlayer) // makes one player fail
	}{
		{"maker fails", func(a, b *fakePlayer) { a.makeErr = errBoom }},
		{"breaker fails", func(a, b *fakePlayer) { b.guessErr = errBoom }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alice, bob, seats := newPair()
			tt.setup(alice, bob)
			rec := &recorder{}
			m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, rec)

			st, err := m.Play(context.Background())
			if !errors.Is(err, errBoom) {
				t.Fatalf("Play error = %v, want one wrapping errBoom", err)
			}
			if !m.Over() || st.Winner != -1 || len(st.Results) != 0 {
				t.Errorf("over=%v winner=%d results=%d, want over, -1, 0", m.Over(), st.Winner, len(st.Results))
			}
			if last := rec.events[len(rec.events)-1]; last != "match winner=-1" {
				t.Errorf("last event = %q, want the match-ended event", last)
			}
		})
	}
}

// TestInvalidSecretFromMaker checks that a buggy maker returning an invalid
// secret causes an error, not a panic or a broken round.
func TestInvalidSecretFromMaker(t *testing.T) {
	alice, _, seats := newPair()
	alice.secret = code.Code{} // five Blacks: invalid
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)
	if _, err := m.Play(context.Background()); err == nil {
		t.Fatal("Play accepted an invalid secret")
	}
}

// TestContextCanceled checks that a cancelled context (Ctrl-C) stops play
// and the error says why.
//
// Go note: context.WithCancel returns a context and a cancel function.
// Calling cancel marks the context done; code that checks ctx.Err() then
// sees context.Canceled and can stop cleanly.
func TestContextCanceled(t *testing.T) {
	_, _, seats := newPair()
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Play(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Play error = %v, want context.Canceled", err)
	}
}

// TestContextCanceledMidRound checks that Ctrl-C during a round, not just
// before it, stops play at the next guess.
func TestContextCanceledMidRound(t *testing.T) {
	_, bob, seats := newPair()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Go note: always call cancel eventually, to release the context's resources.
	bob.cancelOn, bob.cancel = 3, cancel

	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)
	if _, err := m.Play(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Play error = %v, want context.Canceled", err)
	}
}

// TestBuggyBreakerRepeat checks that a breaker returning a repeated guess
// is treated as a bug: play stops with a *RepeatGuessError inside the
// error, rather than looping or silently using a row.
func TestBuggyBreakerRepeat(t *testing.T) {
	_, bob, seats := newPair()
	bob.repeatOn = 2
	m, _ := NewMatch(Config{FirstMaker: FirstMakerSeat0}, seats, nil, nil)

	_, err := m.Play(context.Background())
	var rep *RepeatGuessError
	if !errors.As(err, &rep) {
		t.Errorf("Play error = %v, want one containing *RepeatGuessError", err)
	}
}

// TestStandingsReturnsCopy checks that callers can't change the recorded
// results through the slice they're given.
func TestStandingsReturnsCopy(t *testing.T) {
	_, bob, seats := newPair()
	bob.solveAt[1] = 1
	m, _ := NewMatch(Config{Mode: ModeSingle, FirstMaker: FirstMakerSeat0}, seats, nil, nil)
	m.Play(context.Background())

	st := m.Standings()
	st.Results[0].Points = 999
	if m.Standings().Results[0].Points == 999 {
		t.Error("changing returned Results changed the match's record")
	}
}

// TestEnumStrings checks the readable names of the small enum types.
func TestEnumStrings(t *testing.T) {
	tests := []struct {
		got, want string
	}{
		{ModeMatch.String(), "match"},
		{ModeSingle.String(), "single game"},
		{Mode(9).String(), "Mode(9)"},
		{PegBlue.String(), "Blue"},
		{PegRed.String(), "Red"},
		{PegColor(9).String(), "PegColor(9)"},
		{FirstMakerCoinFlip.String(), "coin flip"},
		{FirstMakerSeat0.String(), "seat 1"},
		{FirstMakerSeat1.String(), "seat 2"},
		{FirstMaker(9).String(), "FirstMaker(9)"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

// infoRecorder keeps the RoundInfo of every event it sees.
//
// Go note: embedding NopObserver gives infoRecorder all four Observer
// methods; it then overrides only the two it cares about.
type infoRecorder struct {
	NopObserver
	infos []RoundInfo
}

func (r *infoRecorder) RoundStarted(info RoundInfo)       { r.infos = append(r.infos, info) }
func (r *infoRecorder) TurnPlayed(info RoundInfo, _ Turn) { r.infos = append(r.infos, info) }

// TestEasyModeReachesObserver checks that Config.Easy is passed on in every
// RoundInfo, so the screen knows to draw the marks by position, and that
// it's off by default.
func TestEasyModeReachesObserver(t *testing.T) {
	for _, easy := range []bool{false, true} {
		_, bob, seats := newPair()
		bob.solveAt[1] = 2
		rec := &infoRecorder{}
		m, _ := NewMatch(Config{Mode: ModeSingle, FirstMaker: FirstMakerSeat0, Easy: easy}, seats, nil, rec)
		m.Play(context.Background())
		if len(rec.infos) != 3 {
			t.Fatalf("easy=%v: got %d events, want 3", easy, len(rec.infos))
		}
		for _, info := range rec.infos {
			if info.Easy != easy {
				t.Errorf("easy=%v: RoundInfo.Easy = %v", easy, info.Easy)
			}
		}
	}
}

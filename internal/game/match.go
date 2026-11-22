package game

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"

	"codebreaker/internal/code"
)

// RoundResult is the record of one finished round.
type RoundResult struct {
	// Number is the 1-based round number.
	Number int
	// MakerIdx and BreakerIdx are the seat indexes (0 or 1) in each role.
	MakerIdx, BreakerIdx int
	// Rows is how many guesses were played.
	Rows int
	// Solved reports whether the Breaker cracked the code.
	Solved bool
	// Secret is the code that was being guessed, revealed at round end.
	Secret code.Code
	// Points is what the Breaker earned (see Points). The Maker earns none.
	Points int
}

// Standings is the state of a match: both seats with their scores, every
// finished round, and the winner.
type Standings struct {
	// Seats holds both players and their current scores.
	Seats [2]Seat
	// Results lists the finished rounds in order.
	Results []RoundResult
	// Winner is the seat index (0 or 1) that reached TargetScore, or -1 if
	// nobody has: the match is still going, was quit, or was a Single game.
	Winner int
}

// Match runs a game between two seats: one round in ModeSingle, or rounds
// with alternating roles until a player reaches TargetScore in ModeMatch.
//
// Create one with NewMatch, then call Play to run it all, or PlayRound to
// run one round at a time.
type Match struct {
	cfg      Config
	seats    [2]Seat
	makerIdx int // the seat making the code in the next (or current) round
	obs      Observer
	results  []RoundResult
	over     bool
}

// NewMatch checks the setup and prepares a match. It doesn't start play.
//
// rng is only used for the coin flip and may be nil when cfg.FirstMaker
// names a seat. obs may be nil if nothing needs to watch. Any scores
// already in seats are reset to 0. Setup problems return an error that
// wraps ErrInvalidSetup.
func NewMatch(cfg Config, seats [2]Seat, rng *rand.Rand, obs Observer) (*Match, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	// Go note: `for i := range seats` loops over the indexes 0 and 1. We
	// index into the array (seats[i].Score = 0) rather than using a
	// `for _, s := range` copy, because changing the copy s would not
	// change the array.
	for i := range seats {
		if seats[i].Player == nil {
			return nil, fmt.Errorf("%w: seat %d has no player", ErrInvalidSetup, i+1)
		}
		if seats[i].Name == "" {
			return nil, fmt.Errorf("%w: seat %d has no name", ErrInvalidSetup, i+1)
		}
		if !seats[i].Peg.valid() {
			return nil, fmt.Errorf("%w: seat %d has unknown peg color %v", ErrInvalidSetup, i+1, seats[i].Peg)
		}
		seats[i].Score = 0
	}
	if seats[0].Peg == seats[1].Peg {
		return nil, fmt.Errorf("%w: both seats use %v", ErrInvalidSetup, seats[0].Peg)
	}

	var first int
	switch cfg.FirstMaker {
	case FirstMakerSeat0:
		first = 0
	case FirstMakerSeat1:
		first = 1
	default: // FirstMakerCoinFlip
		if rng == nil {
			return nil, fmt.Errorf("%w: coin flip needs a random generator", ErrInvalidSetup)
		}
		first = rng.IntN(2) // 0 or 1, each equally likely
	}

	if obs == nil {
		obs = NopObserver{}
	}
	return &Match{cfg: cfg, seats: seats, makerIdx: first, obs: obs}, nil
}

// MakerIdx returns the seat that makes the code in the next round to be
// played. Right after NewMatch, it's the result of the coin flip.
func (m *Match) MakerIdx() int { return m.makerIdx }

// Seats returns a copy of both seats with their current scores.
//
// Go note: [2]Seat is an array, so returning it copies it. Callers can't
// change the match's seats through it.
func (m *Match) Seats() [2]Seat { return m.seats }

// Over reports whether the match has ended, normally or because of an
// error. Once over, PlayRound returns ErrMatchOver.
func (m *Match) Over() bool { return m.over }

// Standings returns the current scores, finished rounds and winner.
func (m *Match) Standings() Standings {
	winner := -1
	if m.cfg.Mode == ModeMatch {
		for i, s := range m.seats {
			if s.Score >= TargetScore {
				winner = i
			}
		}
	}
	return Standings{
		Seats:   m.seats,
		Results: slices.Clone(m.results),
		Winner:  winner,
	}
}

// Play runs rounds until the match is over and returns the final standings.
//
// If a player returns an error (such as quitting) or ctx is cancelled, play
// stops and Play returns the standings so far along with that error. The
// Observer's MatchEnded is called in both cases, so the screen can always
// show a summary (D16).
func (m *Match) Play(ctx context.Context) (Standings, error) {
	var err error
	for !m.over {
		if _, err = m.PlayRound(ctx); err != nil {
			break
		}
	}
	st := m.Standings()
	m.obs.MatchEnded(st)
	return st, err
}

// PlayRound plays one full round: the maker sets a secret, the breaker
// guesses until solved or out of rows, and the breaker's points are added.
// Then the match either ends (Single mode, or a score reached
// TargetScore) or swaps roles for the next round.
//
// On any error the match is marked over and the round isn't recorded.
// Errors from players are wrapped with the round number and player name;
// use errors.Is or errors.As to inspect them.
func (m *Match) PlayRound(ctx context.Context) (RoundResult, error) {
	if m.over {
		return RoundResult{}, ErrMatchOver
	}

	result, err := m.playRound(ctx)
	if err != nil {
		// Any failure ends the match: a half-played round can't be scored,
		// and the usual cause is a player quitting.
		m.over = true
		return RoundResult{}, err
	}

	// Go note: `&m.seats[i]` takes the address of an array element, so
	// adding to breaker.Score changes the seat stored in the match, not a
	// copy.
	breaker := &m.seats[result.BreakerIdx]
	breaker.Score += result.Points
	m.results = append(m.results, result)
	m.obs.RoundEnded(result, m.seats)

	switch {
	case m.cfg.Mode == ModeSingle:
		m.over = true
	case breaker.Score >= TargetScore:
		// Only the breaker scores, so only the breaker can have just
		// reached the target. The match ends right away (D5, D15).
		m.over = true
	default:
		m.makerIdx = 1 - m.makerIdx // swap roles: 0 → 1, 1 → 0
	}
	return result, nil
}

// playRound does the work of one round and reports what happened. Scoring
// and deciding what comes next are left to PlayRound.
func (m *Match) playRound(ctx context.Context) (RoundResult, error) {
	number := len(m.results) + 1
	makerIdx, breakerIdx := m.makerIdx, 1-m.makerIdx
	maker, breaker := m.seats[makerIdx], m.seats[breakerIdx]

	info := RoundInfo{
		Number:  number,
		Mode:    m.cfg.Mode,
		MaxRows: MaxRows,
		Maker:   maker,
		Breaker: breaker,
	}

	// Go note: ctx.Err() is nil while the context is live, and
	// context.Canceled (or DeadlineExceeded) once it's done. Checking it
	// before each slow step makes Ctrl-C take effect promptly.
	if err := ctx.Err(); err != nil {
		return RoundResult{}, fmt.Errorf("round %d: %w", number, err)
	}
	m.obs.RoundStarted(info)

	secret, err := maker.Player.MakeSecret(ctx, info)
	if err != nil {
		return RoundResult{}, fmt.Errorf("round %d: %s making secret: %w", number, maker.Name, err)
	}
	round, err := NewRound(secret)
	if err != nil {
		// A real player always returns a valid code, so this is a bug.
		return RoundResult{}, fmt.Errorf("round %d: %s made a bad secret: %w", number, maker.Name, err)
	}

	for round.Status() == InProgress {
		if err := ctx.Err(); err != nil {
			round.Abandon()
			return RoundResult{}, fmt.Errorf("round %d: %w", number, err)
		}
		guess, err := breaker.Player.Guess(ctx, info, round.Turns())
		if err != nil {
			round.Abandon()
			return RoundResult{}, fmt.Errorf("round %d: %s guessing: %w", number, breaker.Name, err)
		}
		turn, err := round.Submit(guess)
		if err != nil {
			// Human players check their input before returning it, and
			// the computer never repeats, so this is a bug.
			round.Abandon()
			return RoundResult{}, fmt.Errorf("round %d: %s made a bad guess: %w", number, breaker.Name, err)
		}
		m.obs.TurnPlayed(info, turn)
	}

	solved := round.Status() == Solved
	return RoundResult{
		Number:     number,
		MakerIdx:   makerIdx,
		BreakerIdx: breakerIdx,
		Rows:       round.RowsUsed(),
		Solved:     solved,
		Secret:     secret,
		Points:     Points(round.RowsUsed(), solved),
	}, nil
}

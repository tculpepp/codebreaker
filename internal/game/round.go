package game

import (
	"fmt"
	"slices"

	"codebreaker/internal/code"
)

// RoundStatus says whether a round is still being played and, if not, how
// it ended.
type RoundStatus int

// The possible round states. A round starts InProgress and moves to exactly
// one of the other three, after which it never changes:
//
//	InProgress ──5 exact──────▶ Solved
//	InProgress ──row 12 used──▶ OutOfRows
//	InProgress ──player quit──▶ Abandoned
const (
	InProgress RoundStatus = iota
	Solved
	OutOfRows
	Abandoned
)

// String returns a readable name for the status, such as "out of rows".
//
// Go note: Go has no built-in enum type. The usual pattern is a named int
// type, constants made with iota, and a String method so the values print
// as words instead of numbers.
func (s RoundStatus) String() string {
	switch s {
	case InProgress:
		return "in progress"
	case Solved:
		return "solved"
	case OutOfRows:
		return "out of rows"
	case Abandoned:
		return "abandoned"
	default:
		return fmt.Sprintf("RoundStatus(%d)", int(s))
	}
}

// Turn is one row on the board: a guess and the feedback it got.
type Turn struct {
	// Row is the 1-based row number (1..MaxRows).
	Row int
	// Guess is the code the Breaker played.
	Guess code.Code
	// Feedback is the ●/△ result for that guess.
	Feedback code.Feedback
	// Marks is the same result position by position, for easy mode:
	// Marks[i] is about Guess[i]. It's always filled in; the screen shows
	// it only when RoundInfo.Easy is set.
	Marks [code.Length]code.Mark
}

// Round is one secret code and the guesses made against it.
//
// All fields are unexported, so code outside this package can only use the
// methods. In particular, nothing outside can read the secret while the
// round is in progress, and nothing can change the history.
//
// Go note: in Go, visibility is decided by the first letter of a name.
// Uppercase (Submit) is exported, meaning other packages can use it.
// Lowercase (secret) is only visible inside package game. There are no
// "public"/"private" keywords.
type Round struct {
	secret code.Code
	turns  []Turn
	status RoundStatus
}

// NewRound starts a round with the given secret. It returns an error if the
// secret breaks the code rules (for example a repeated color).
//
// Go note: Go has no constructors. A function named New<Type> that checks
// its inputs and returns a ready-to-use value is the convention. It returns
// a pointer (*Round) because a Round changes as guesses are made, and every
// caller must see the same, updated round rather than a copy.
func NewRound(secret code.Code) (*Round, error) {
	if err := secret.Validate(); err != nil {
		return nil, fmt.Errorf("invalid secret: %w", err)
	}
	return &Round{
		secret: secret,
		// Room for every row up front, so append never has to grow it.
		turns:  make([]Turn, 0, MaxRows),
		status: InProgress,
	}, nil
}

// Submit plays one guess. It returns the new Turn, with its feedback, and
// updates the round's status when the guess solves the code or uses the
// last row.
//
// A guess that breaks the code rules, or repeats an earlier guess this
// round, is refused with an error and does not use a row. After the round
// has ended, Submit returns ErrRoundOver.
//
// Go note: `(r *Round)` is a pointer receiver. The method gets the address
// of the Round, so its changes (appending a turn, updating the status) are
// made to the caller's Round. With a value receiver `(r Round)`, the method
// would change a throwaway copy and the round would never advance.
func (r *Round) Submit(guess code.Code) (Turn, error) {
	if r.status != InProgress {
		return Turn{}, ErrRoundOver
	}
	if err := guess.Validate(); err != nil {
		return Turn{}, err
	}
	if err := CheckRepeat(r.turns, guess); err != nil {
		return Turn{}, err
	}

	turn := Turn{
		Row:      len(r.turns) + 1,
		Guess:    guess,
		Feedback: code.Score(r.secret, guess),
		Marks:    code.Marks(r.secret, guess),
	}
	r.turns = append(r.turns, turn)

	// Solving takes priority: cracking the code on row 12 counts as Solved,
	// not OutOfRows.
	switch {
	case turn.Feedback.Solved():
		r.status = Solved
	case len(r.turns) == MaxRows:
		r.status = OutOfRows
	}
	return turn, nil
}

// Abandon ends a round early, for example when a player quits. It does
// nothing if the round has already ended, so how a finished round ended is
// never overwritten.
func (r *Round) Abandon() {
	if r.status == InProgress {
		r.status = Abandoned
	}
}

// Status reports whether the round is in progress or how it ended.
func (r *Round) Status() RoundStatus { return r.status }

// RowsUsed returns how many guesses have been played.
func (r *Round) RowsUsed() int { return len(r.turns) }

// RowsLeft returns how many guesses remain before the row limit.
func (r *Round) RowsLeft() int { return MaxRows - len(r.turns) }

// Turns returns a copy of the guesses played so far, oldest first.
//
// Go note: returning r.turns directly would hand out a slice that shares
// its underlying array with the Round. A caller writing to it would change
// the game's history. slices.Clone gives the caller their own copy.
func (r *Round) Turns() []Turn { return slices.Clone(r.turns) }

// Secret returns the secret code and true once the round is over. While the
// round is in progress it returns the zero Code and false, so nobody can
// peek.
func (r *Round) Secret() (code.Code, bool) {
	if r.status == InProgress {
		return code.Code{}, false
	}
	return r.secret, true
}

// CheckRepeat returns a *RepeatGuessError if guess already appears in
// history, or nil if it's new (requirements.md D14).
//
// Human players call it before submitting, so they can re-type without
// losing a row. Round.Submit also calls it, as a safety net.
func CheckRepeat(history []Turn, guess code.Code) error {
	for _, t := range history {
		// Code is an array, so == compares all 5 colors.
		if t.Guess == guess {
			return &RepeatGuessError{Guess: guess, Row: t.Row}
		}
	}
	return nil
}

package game

import "fmt"

// Mode is the kind of game being played.
type Mode int

// The game modes (requirements.md "Game Modes").
//
// Go note: ModeMatch is listed first so it's the zero value. A Config{}
// with nothing filled in then means "a full match", the main mode. Choosing
// constants so that the zero value is a sensible default is a common Go
// design habit ("make the zero value useful").
const (
	// ModeMatch plays rounds, swapping roles, until someone reaches
	// TargetScore (D5).
	ModeMatch Mode = iota
	// ModeSingle plays exactly one round (D13).
	ModeSingle
)

// String returns "match" or "single game".
func (m Mode) String() string {
	switch m {
	case ModeMatch:
		return "match"
	case ModeSingle:
		return "single game"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// valid reports whether m is one of the defined modes.
func (m Mode) valid() bool { return m == ModeMatch || m == ModeSingle }

// PegColor is a player's scoring-peg color, which also identifies them on
// screen (D11).
type PegColor int

// The two scoring-peg colors from the box.
const (
	PegBlue PegColor = iota
	PegRed
)

// String returns "Blue" or "Red".
func (p PegColor) String() string {
	switch p {
	case PegBlue:
		return "Blue"
	case PegRed:
		return "Red"
	default:
		return fmt.Sprintf("PegColor(%d)", int(p))
	}
}

// valid reports whether p is one of the defined peg colors.
func (p PegColor) valid() bool { return p == PegBlue || p == PegRed }

// FirstMaker says how the first Code Maker is chosen.
type FirstMaker int

// The ways to choose the first Code Maker. The zero value is the coin flip,
// which is what the box rules use (D6).
const (
	// FirstMakerCoinFlip picks a seat at random (needs a random generator).
	FirstMakerCoinFlip FirstMaker = iota
	// FirstMakerSeat0 makes the first seat the Code Maker.
	FirstMakerSeat0
	// FirstMakerSeat1 makes the second seat the Code Maker.
	FirstMakerSeat1
)

// String returns "coin flip", "seat 1" or "seat 2" (counting seats from 1,
// as a player would).
func (f FirstMaker) String() string {
	switch f {
	case FirstMakerCoinFlip:
		return "coin flip"
	case FirstMakerSeat0:
		return "seat 1"
	case FirstMakerSeat1:
		return "seat 2"
	default:
		return fmt.Sprintf("FirstMaker(%d)", int(f))
	}
}

// Config holds the choices made before a match starts. Its zero value,
// Config{}, means a full match with a coin flip for the first Code Maker.
type Config struct {
	// Mode is a full match or a single round.
	Mode Mode
	// FirstMaker says who makes the first code.
	FirstMaker FirstMaker
	// Easy shows each key peg in the same position as the guess peg it
	// scores, instead of only counting them (the Easy difficulty). It changes
	// what the screen shows, not the scoring.
	Easy bool
}

// validate checks that every field holds a defined value.
func (c Config) validate() error {
	if !c.Mode.valid() {
		return fmt.Errorf("%w: unknown mode %v", ErrInvalidSetup, c.Mode)
	}
	switch c.FirstMaker {
	case FirstMakerCoinFlip, FirstMakerSeat0, FirstMakerSeat1:
		return nil
	default:
		return fmt.Errorf("%w: unknown first maker %v", ErrInvalidSetup, c.FirstMaker)
	}
}

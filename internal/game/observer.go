package game

// Observer is told about everything that happens during a match, in order.
// The terminal UI implements it to draw the board. Tests implement it to
// record events.
//
// The match calls these methods synchronously: the next step waits until
// the observer returns.
type Observer interface {
	// RoundStarted is called before the Code Maker sets the secret.
	RoundStarted(info RoundInfo)
	// TurnPlayed is called after each accepted guess.
	TurnPlayed(info RoundInfo, turn Turn)
	// RoundEnded is called once a round is solved or out of rows, with
	// both seats' updated scores.
	RoundEnded(result RoundResult, seats [2]Seat)
	// MatchEnded is called once when play stops, normally or because of
	// an error such as the player quitting.
	MatchEnded(standings Standings)
}

// NopObserver ignores every event. NewMatch uses it when no observer is
// given, so the match code never has to check for nil.
//
// Go note: an empty struct, struct{}, takes no memory. It's handy for types
// that only exist to carry methods. Embedding NopObserver in another struct
// also gives that struct all four methods, so a test fake can override just
// the ones it cares about.
type NopObserver struct{}

// RoundStarted does nothing.
func (NopObserver) RoundStarted(RoundInfo) {}

// TurnPlayed does nothing.
func (NopObserver) TurnPlayed(RoundInfo, Turn) {}

// RoundEnded does nothing.
func (NopObserver) RoundEnded(RoundResult, [2]Seat) {}

// MatchEnded does nothing.
func (NopObserver) MatchEnded(Standings) {}

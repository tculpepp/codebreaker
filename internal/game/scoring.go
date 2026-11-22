package game

// Rules fixed by the box (requirements.md D3, D5).
//
// Go note: these are untyped constants. They have no fixed type until used,
// so MaxRows works as an int, an int64, or a float64 without conversions.
const (
	// MaxRows is the most guesses a Code Breaker gets in one round.
	MaxRows = 12
	// TargetScore is the score that wins a match.
	TargetScore = 30
)

// Points returns how far the Code Breaker advances on the scoring track
// after a round (requirements.md "Scoring", D4):
//
//	solved in rows 1–4   → 12
//	solved in rows 5–6   → 10
//	solved in rows 7–8   →  8
//	solved in rows 9–10  →  6
//	solved in rows 11–12 →  4
//	not solved           →  0
//
// Row counts outside 1..MaxRows can't happen in a real round, so they also
// score 0 rather than inventing points.
//
// Go note: with only one scoring rule there's no need for a Scorer
// interface; a plain function is simpler to read and test. A good habit in
// Go is to add an interface only once there are two or more real
// implementations, or when a test needs to swap one in.
func Points(rows int, solved bool) int {
	if !solved {
		return 0
	}
	// Go note: a switch with no value after `switch` ("tagless switch")
	// tests each case as a true/false condition, top to bottom, and runs
	// only the first that's true. Go cases don't fall through to the next
	// one, so no `break` is needed.
	switch {
	case rows < 1 || rows > MaxRows:
		return 0
	case rows <= 4:
		return 12
	case rows <= 6:
		return 10
	case rows <= 8:
		return 8
	case rows <= 10:
		return 6
	default: // rows 11–12
		return 4
	}
}

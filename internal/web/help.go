package web

import (
	"fmt"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// helpView is the help sheet: the rules in short, the controls, and the
// scoring table, worked out from game.Bracket and game.Points so it can't
// disagree with the rules.
func helpView() View {
	// One line per scoring range, so a range like "1–4" never wraps in
	// the middle on a narrow screen.
	var table []string
	for row := 1; row <= game.MaxRows; {
		lo, hi := game.Bracket(row)
		table = append(table, fmt.Sprintf("Rows %d–%d: %d points", lo, hi, game.Points(lo, true)))
		row = hi + 1
	}
	lines := []string{
		fmt.Sprintf("The secret is %d different colors, chosen from %d.", code.Length, code.NumColors),
		"Tap colors to build a guess, then Submit. Undo takes back the last peg.",
		"On a keyboard: type the color letters, Backspace to undo, Enter to submit.",
		"After each guess: ● a right color in the right spot, △ a right color in the wrong spot.",
		"The symbols don't say which pegs they mean, except on Easy, where each sits in the spot of its peg.",
		fmt.Sprintf("You have %d rows. Solve sooner to score more:", game.MaxRows),
	}
	lines = append(lines, table...)
	lines = append(lines, fmt.Sprintf("The first player to reach %d points wins a match.", game.TargetScore))
	return View{
		Screen: ScreenHelp,
		Title:  "How to play",
		Lines:  lines,
	}
}

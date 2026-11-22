package ui

import (
	"fmt"
	"strings"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// Welcome draws the title banner shown when the game starts.
func (r *Renderer) Welcome() {
	var b strings.Builder
	b.WriteString("\n CODEBREAKER\n")
	fmt.Fprintf(&b, " Break the secret code: %d different colors, chosen from %d.\n",
		code.Length, code.NumColors)
	b.WriteString(" Type ? for help or q to quit at any prompt.\n")
	r.write(b.String())
}

// Help draws the rules in short: what a code is, how to type a guess, what
// the feedback means, and how points are scored.
func (r *Renderer) Help() {
	var b strings.Builder
	b.WriteString("\n How to play\n")
	fmt.Fprintf(&b, "   The secret is %d different colors, chosen from:\n", code.Length)
	fmt.Fprintf(&b, "     %s\n", colorKey())
	fmt.Fprintf(&b, "   Type a guess as %d letters: RGYBK, r g y b k or R,G,Y,B,K.\n", code.Length)
	b.WriteString("   After each guess:\n")
	fmt.Fprintf(&b, "     %s a right color in the right spot\n", symExact)
	fmt.Fprintf(&b, "     %s a right color in the wrong spot\n", symPartial)
	b.WriteString("     The symbols don't say which pegs they mean.\n")
	fmt.Fprintf(&b, "   You have %d rows. Solve sooner to score more:\n", game.MaxRows)
	fmt.Fprintf(&b, "     rows %s\n", scoringTable())
	fmt.Fprintf(&b, "   The first player to reach %d points wins a match.\n", game.TargetScore)
	b.WriteString("   Type q to quit.\n")
	r.write(b.String())
}

// Menu draws a title and a numbered list of choices, counting from 1.
// Read the answer with ParseChoice.
func (r *Renderer) Menu(title string, choices []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "\n %s\n", title)
	for i, c := range choices {
		fmt.Fprintf(&b, "   %d) %s\n", i+1, c)
	}
	r.write(b.String())
}

// Goodbye draws the farewell shown when the player leaves the game.
func (r *Renderer) Goodbye() {
	r.write("\n Thanks for playing!\n")
}

// scoringTable lists every scoring bracket and its points:
// "1–4: 12 · 5–6: 10 · ...". It's built from bracket, and so from
// game.Points.
func scoringTable() string {
	var parts []string
	// Go note: Go's only loop keyword is for. This form, with no
	// condition-update in the header, leaves the update to the body, which
	// jumps to the row after each bracket.
	for row := 1; row <= game.MaxRows; {
		lo, hi := bracket(row)
		parts = append(parts, fmt.Sprintf("%d–%d: %d", lo, hi, game.Points(lo, true)))
		row = hi + 1
	}
	return strings.Join(parts, " · ")
}

package web

import (
	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// Screen names which screen the page should draw.
//
// Go note: Screen is a named string type rather than an int with iota,
// because its values travel to JavaScript as text. "board" in the JSON is
// readable in the browser's developer tools, where a 2 wouldn't be.
type Screen string

// The screens the page can draw.
const (
	ScreenName    Screen = "name"    // asking for a player's name
	ScreenMenu    Screen = "menu"    // a numbered list of choices
	ScreenBoard   Screen = "board"   // the game board, with or without input
	ScreenSecret  Screen = "secret"  // a Maker building a secret code
	ScreenHandoff Screen = "handoff" // "pass the device", hiding the secret (D21)
	ScreenSummary Screen = "summary" // the end of a round
	ScreenResult  Screen = "result"  // the end of a match or game
	ScreenHelp    Screen = "help"    // the rules, over the current screen
)

// View is everything the page needs to draw one screen. It's sent as JSON
// after every change. The page keeps no game state of its own: drawing the
// latest View is always right.
//
// Go note: the `json:"..."` after each field is a struct tag. encoding/json
// reads it to name the field in JSON ("screen" rather than "Screen").
// omitempty leaves the field out when it holds its zero value, such as ""
// or a nil pointer, which keeps each View small.
type View struct {
	// Screen says which screen to draw.
	Screen Screen `json:"screen"`
	// Title is the screen's heading, such as "Main menu".
	Title string `json:"title,omitempty"`
	// Lines are short pieces of text: the line-up, a summary, help.
	Lines []string `json:"lines,omitempty"`
	// Menu holds the choices on a menu screen.
	//
	// Go note: a pointer field is how a struct says "this part may be
	// missing". It's nil on screens without a menu, and omitempty then
	// leaves it out of the JSON altogether.
	Menu *MenuView `json:"menu,omitempty"`
	// Board is the game board, on the board and secret screens.
	Board *BoardView `json:"board,omitempty"`
	// Message is a friendly error, shown above the input.
	Message string `json:"message,omitempty"`
	// Input says which controls are active.
	Input InputView `json:"input"`
}

// MenuView is a list of choices. The page sends back the 0-based index of
// the one tapped, in an Event of kind "choose".
type MenuView struct {
	Choices []string `json:"choices"`
}

// InputView says which controls the page should enable. Anything not
// listed is shown disabled or hidden.
type InputView struct {
	Palette bool `json:"palette,omitempty"` // the 8 color buttons and the color keys
	Undo    bool `json:"undo,omitempty"`    // remove the last peg from the draft
	Submit  bool `json:"submit,omitempty"`  // play the draft (guess or secret)
	Skip    bool `json:"skip,omitempty"`    // show the computer's guesses at once
	Ready   bool `json:"ready,omitempty"`   // "I'm ready" on the handoff screen
	Text    bool `json:"text,omitempty"`    // a text box, for names
}

// BoardView is the game board: who plays, the scores, every row so far,
// and the row being built.
type BoardView struct {
	// Title is "Match · Round 3" or "Single game · Round 1".
	Title string `json:"title"`
	// Maker and Breaker are this round's players.
	Maker   SeatView `json:"maker"`
	Breaker SeatView `json:"breaker"`
	// Scores lists both players in a match, Blue first, so the scores
	// stay in the same place on every screen. It's empty in a Single game,
	// which has no match total (D13).
	Scores []SeatView `json:"scores,omitempty"`
	// Target is the winning score, in a match.
	Target int `json:"target,omitempty"`
	// Rows are the guesses played so far, oldest first.
	Rows []RowView `json:"rows"`
	// Draft is the row being built: always code.Length slots, empty ones
	// with no letter. It's missing when no one is building a row, such as
	// while the computer breaks the code.
	Draft []PegView `json:"draft,omitempty"`
	// Secret is the code, revealed when a round ends. It's missing until
	// then, so the page can't show it early.
	Secret []PegView `json:"secret,omitempty"`
	// MaxRows is the row limit (12).
	MaxRows int `json:"maxRows"`
	// Easy says the feedback is by position (D18), so the page can say so.
	Easy bool `json:"easy,omitempty"`
}

// SeatView is a player as the page shows them.
type SeatView struct {
	Name  string `json:"name"`
	Peg   string `json:"peg"` // "blue" or "red", used as a CSS class
	Score int    `json:"score"`
}

// RowView is one played row: its number, the guess and the feedback.
type RowView struct {
	Row   int       `json:"row"`
	Guess []PegView `json:"guess"`
	// Feedback is code.Length marks, each "exact", "partial" or "none":
	// sorted on Normal, by position on Easy.
	Feedback []string `json:"feedback"`
}

// PegView is one code peg. The page colors it by Letter and labels it
// with Name for screen readers. The zero PegView is an empty slot.
type PegView struct {
	Letter string `json:"letter,omitempty"`
	Name   string `json:"name,omitempty"`
}

// pegView describes one color as a peg.
func pegView(c code.Color) PegView {
	return PegView{Letter: string(c.Abbrev()), Name: c.Name()}
}

// pegViews describes a whole code, peg by peg.
func pegViews(c code.Code) []PegView {
	pegs := make([]PegView, 0, code.Length)
	for _, color := range c {
		pegs = append(pegs, pegView(color))
	}
	return pegs
}

// markNames turns marks into the words the page expects: "exact",
// "partial" or "none" (from code.Mark's String method).
func markNames(marks [code.Length]code.Mark) []string {
	names := make([]string, 0, code.Length)
	for _, m := range marks {
		names = append(names, m.String())
	}
	return names
}

// seatView describes a seat. The peg color is lowercase because the page
// uses it as a CSS class name.
func seatView(s game.Seat) SeatView {
	peg := "blue"
	if s.Peg == game.PegRed {
		peg = "red"
	}
	return SeatView{Name: s.Name, Peg: peg, Score: s.Score}
}

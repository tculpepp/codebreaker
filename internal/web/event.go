package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"codebreaker/internal/code"
)

// ErrBadEvent is wrapped by ParseEvent for anything the page sent that
// isn't a valid event. It would be a bug in the page, not a player's
// mistake, so it's never shown to the player.
var ErrBadEvent = errors.New("web: bad event")

// EventKind says what the player did.
type EventKind string

// The kinds of event the page sends.
const (
	KindColor  EventKind = "color"  // tapped a color, or typed its letter (Letter)
	KindUndo   EventKind = "undo"   // Undo, or Backspace
	KindSubmit EventKind = "submit" // Submit, or Enter
	KindChoose EventKind = "choose" // tapped a menu choice (Choice, from 0)
	KindText   EventKind = "text"   // entered text, such as a name (Text)
	KindReady  EventKind = "ready"  // "I'm ready" after a handoff
	KindSkip   EventKind = "skip"   // skip the computer's paced guesses
	KindHelp   EventKind = "help"   // open help
	KindClose  EventKind = "close"  // close help
	KindQuit   EventKind = "quit"   // stop the game and show the standings
)

// kinds lists every valid EventKind, for checking.
var kinds = []EventKind{KindColor, KindUndo, KindSubmit, KindChoose, KindText,
	KindReady, KindSkip, KindHelp, KindClose, KindQuit}

// Event is one thing the player did on the page. Only the field that its
// Kind uses is set.
type Event struct {
	Kind   EventKind `json:"kind"`
	Letter string    `json:"letter,omitempty"` // for KindColor, always uppercase after parsing
	Choice int       `json:"choice,omitempty"` // for KindChoose, counting from 0
	Text   string    `json:"text,omitempty"`   // for KindText

	// color is Letter as a code.Color, worked out by ParseEvent.
	//
	// Go note: encoding/json only reads and writes exported fields, so
	// this lowercase field never appears in the JSON. It's filled in after
	// decoding.
	color code.Color
}

// ParseEvent reads one event sent by the page as JSON and checks it. A
// color letter may be either case, as typed on a keyboard. Anything
// invalid returns an error wrapping ErrBadEvent.
func ParseEvent(data []byte) (Event, error) {
	var ev Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return Event{}, fmt.Errorf("%w: %v", ErrBadEvent, err)
	}
	if !isKind(ev.Kind) {
		return Event{}, fmt.Errorf("%w: unknown kind %q", ErrBadEvent, ev.Kind)
	}
	switch ev.Kind {
	case KindColor:
		// Go note: len(s) counts bytes, not characters. utf8.RuneCountInString
		// counts characters, so "é" counts as 1 and is refused as a letter
		// below rather than as "too long".
		r, _ := utf8.DecodeRuneInString(ev.Letter)
		c, ok := code.ColorFromRune(r)
		if utf8.RuneCountInString(ev.Letter) != 1 || !ok {
			return Event{}, fmt.Errorf("%w: %q is not a color letter", ErrBadEvent, ev.Letter)
		}
		ev.color = c
		ev.Letter = strings.ToUpper(ev.Letter)
	case KindChoose:
		if ev.Choice < 0 {
			return Event{}, fmt.Errorf("%w: choice %d", ErrBadEvent, ev.Choice)
		}
	}
	return ev, nil
}

// Color returns the color of a KindColor event from ParseEvent.
func (e Event) Color() code.Color { return e.color }

// isKind reports whether k is one of the defined kinds.
func isKind(k EventKind) bool {
	for _, known := range kinds {
		if k == known {
			return true
		}
	}
	return false
}

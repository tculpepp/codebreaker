package web

import (
	"errors"
	"testing"

	"github.com/tculpepp/codebreaker/internal/code"
)

// TestParseEvent checks the events the page can send, as JSON. Color
// letters may be either case, as on the keyboard.
func TestParseEvent(t *testing.T) {
	valid := []struct {
		json string
		want Event
	}{
		{`{"kind":"color","letter":"r"}`, Event{Kind: KindColor, Letter: "R"}},
		{`{"kind":"color","letter":"K"}`, Event{Kind: KindColor, Letter: "K"}},
		{`{"kind":"undo"}`, Event{Kind: KindUndo}},
		{`{"kind":"submit"}`, Event{Kind: KindSubmit}},
		{`{"kind":"choose","choice":2}`, Event{Kind: KindChoose, Choice: 2}},
		{`{"kind":"text","text":"Ann"}`, Event{Kind: KindText, Text: "Ann"}},
		{`{"kind":"ready"}`, Event{Kind: KindReady}},
		{`{"kind":"skip"}`, Event{Kind: KindSkip}},
		{`{"kind":"help"}`, Event{Kind: KindHelp}},
		{`{"kind":"close"}`, Event{Kind: KindClose}},
		{`{"kind":"quit"}`, Event{Kind: KindQuit}},
	}
	for _, tt := range valid {
		got, err := ParseEvent([]byte(tt.json))
		got.color = 0 // checked by TestEventColor; the JSON fields are checked here
		if err != nil || got != tt.want {
			t.Errorf("ParseEvent(%s) = %+v, %v; want %+v", tt.json, got, err, tt.want)
		}
	}

	invalid := []string{
		`not json`,
		`{"kind":"dance"}`,
		`{}`,
		`{"kind":"color","letter":"X"}`,
		`{"kind":"color","letter":"RG"}`,
		`{"kind":"color"}`,
		`{"kind":"choose","choice":-1}`,
	}
	for _, s := range invalid {
		if _, err := ParseEvent([]byte(s)); !errors.Is(err, ErrBadEvent) {
			t.Errorf("ParseEvent(%s) error = %v, want one wrapping ErrBadEvent", s, err)
		}
	}
}

// TestEventColor checks that a parsed color event gives its color.
func TestEventColor(t *testing.T) {
	ev, _ := ParseEvent([]byte(`{"kind":"color","letter":"o"}`))
	if got := ev.Color(); got != code.Orange {
		t.Errorf("Color() = %v, want Orange", got)
	}
}

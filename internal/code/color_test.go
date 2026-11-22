package code

// Go note: a test file lives in the same folder as the code it tests and its
// name ends in "_test.go". `go build` ignores these files; only `go test`
// compiles them. Because this file declares `package code` (not
// `package code_test`), the tests can also see unexported names. That is
// called an "internal" test.

import "testing"

// TestColorTable checks every color's letter and name against the
// requirements' color table (requirements.md, "Color Palette").
//
// Go note: this is a "table-driven test", the most common test style in Go.
// The cases are a slice of anonymous structs, and one loop runs every case.
// Adding a case means adding one line of data, not a new test function.
func TestColorTable(t *testing.T) {
	tests := []struct {
		color  Color
		abbrev rune // Go note: a rune is a Unicode code point (an alias for int32); 'K' is a rune literal.
		name   string
	}{
		{Black, 'K', "Black"},
		{White, 'W', "White"},
		{Red, 'R', "Red"},
		{Blue, 'B', "Blue"},
		{Green, 'G', "Green"},
		{Yellow, 'Y', "Yellow"},
		{Pink, 'P', "Pink"},
		{Orange, 'O', "Orange"},
	}

	for _, tt := range tests {
		// Go note: t.Run starts a "subtest" with its own name. A failure
		// then reads "TestColorTable/Red", and you can run a single case
		// with: go test -run 'TestColorTable/Red' ./internal/code
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.color.Abbrev(); got != tt.abbrev {
				// Go note: %q prints a rune quoted, like 'K', which makes
				// whitespace or odd characters visible in failure output.
				t.Errorf("Abbrev() = %q, want %q", got, tt.abbrev)
			}
			if got := tt.color.Name(); got != tt.name {
				t.Errorf("Name() = %q, want %q", got, tt.name)
			}
			// String() should match Name(), so fmt.Print shows readable
			// names instead of raw numbers.
			if got := tt.color.String(); got != tt.name {
				t.Errorf("String() = %q, want %q", got, tt.name)
			}
		})
	}
}

// TestNumColors guards the "8 colors" rule (D7). If someone adds or removes
// a color constant, this test fails and points them to the requirements.
func TestNumColors(t *testing.T) {
	if NumColors != 8 {
		t.Fatalf("NumColors = %d, want 8 (requirements: 8 distinct colors)", NumColors)
	}
	if got := len(AllColors()); got != int(NumColors) {
		t.Fatalf("len(AllColors()) = %d, want %d", got, NumColors)
	}
}

// TestAbbrevsAreUnique proves no two colors share a letter. If two did,
// parsing a guess would be ambiguous.
func TestAbbrevsAreUnique(t *testing.T) {
	// Go note: map[rune]Color is a hash map from letter to color. Reading a
	// missing key returns the zero value, and the two-value form
	// `v, ok := m[k]` says whether the key was actually present.
	seen := map[rune]Color{}
	for _, c := range AllColors() {
		if other, ok := seen[c.Abbrev()]; ok {
			t.Errorf("%v and %v share the letter %q", other, c, c.Abbrev())
		}
		seen[c.Abbrev()] = c
	}
}

// TestColorFromRune checks letter-to-color lookup: both cases must work
// (requirements: "case-insensitive"), and anything else must be rejected.
func TestColorFromRune(t *testing.T) {
	tests := []struct {
		name   string
		in     rune
		want   Color
		wantOK bool
	}{
		{"upper case", 'R', Red, true},
		{"lower case", 'r', Red, true},
		{"K is Black, not Blue", 'k', Black, true},
		{"B is Blue", 'B', Blue, true},
		{"unknown letter", 'X', 0, false},
		{"digit", '1', 0, false},
		{"space", ' ', 0, false},
		{"non-ASCII", 'é', 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ColorFromRune(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ColorFromRune(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			// Only compare the color when the lookup succeeded; on failure
			// the returned color is meaningless.
			if ok && got != tt.want {
				t.Errorf("ColorFromRune(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestColorRoundTrip is a small property test: for every color, turning it
// into a letter and back gives the same color.
func TestColorRoundTrip(t *testing.T) {
	for _, c := range AllColors() {
		got, ok := ColorFromRune(c.Abbrev())
		if !ok || got != c {
			t.Errorf("round trip of %v via %q gave (%v, %v)", c, c.Abbrev(), got, ok)
		}
	}
}

// TestValid checks the range check used to reject bad Color values, for
// example a Color built from a raw number with Color(42).
func TestValid(t *testing.T) {
	for _, c := range AllColors() {
		if !c.Valid() {
			t.Errorf("%v.Valid() = false, want true", c)
		}
	}
	if NumColors.Valid() {
		t.Error("NumColors.Valid() = true, want false (it is a count, not a color)")
	}
	if Color(200).Valid() {
		t.Error("Color(200).Valid() = true, want false")
	}
}

// TestInvalidColorStrings proves that printing an out-of-range Color does not
// panic and gives a recognizable placeholder. That helps when debugging.
func TestInvalidColorStrings(t *testing.T) {
	bad := Color(42)
	if got := bad.Abbrev(); got != '?' {
		t.Errorf("Abbrev() = %q, want '?'", got)
	}
	if got, want := bad.String(), "Color(42)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

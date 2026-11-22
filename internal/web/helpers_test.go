package web

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"codebreaker/internal/code"
	"codebreaker/internal/game"
)

// update rewrites testdata/*.golden.json instead of comparing against
// them: `go test ./internal/web -update`, then review with `git diff`.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// checkGolden compares got with testdata/<name>.golden.json, or rewrites
// the file when -update is set. The JSON files are the contract with
// render.js: a change to one is a change the page must handle too.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/web -update` to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// mustParse turns a code string into a code.Code, failing the test on a
// typo in the test itself.
func mustParse(t *testing.T, s string) code.Code {
	t.Helper()
	c, err := code.Parse(s)
	if err != nil {
		t.Fatalf("bad code %q in test: %v", s, err)
	}
	return c
}

// Seats shared by the tests. Players are nil because the board only reads
// names, pegs and scores.
var (
	tom      = game.Seat{Name: "Tom", Peg: game.PegBlue}
	computer = game.Seat{Name: "Computer", Peg: game.PegRed}
)

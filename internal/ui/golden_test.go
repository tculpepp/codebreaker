package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the golden files instead of comparing against them. Run
// `go test ./internal/ui -update` after a deliberate change to the screen
// layout, then review the changed files with `git diff` before committing.
//
// Go note: a test binary accepts flags like any Go program. Declaring one
// with the flag package in a _test.go file adds it to `go test`, and the
// testing package parses it before the tests run.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// checkGolden compares got with testdata/<name>.golden, or rewrites that
// file when -update is set.
//
// Go note: a "golden file" holds the exact expected output of a test. It's
// a good fit for screen layouts: the expected board is easy to read and
// review as a file, where the same text inside a Go string literal would
// be full of \n and hard to see.
func checkGolden(t *testing.T, name, got string) {
	// Go note: t.Helper marks this function as a test helper, so a failure
	// is reported at the line in the test that called it, not here.
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/ui -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- got ---\n%s--- want ---\n%s", path, got, want)
	}
}

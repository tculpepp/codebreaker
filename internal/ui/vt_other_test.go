//go:build !windows

package ui

import (
	"strings"
	"testing"
)

// TestEnableVTOutsideWindows checks that Unix terminals need nothing
// turned on: enableVT always reports success there.
func TestEnableVTOutsideWindows(t *testing.T) {
	if !enableVT(&strings.Builder{}) {
		t.Error("enableVT = false, want true outside Windows")
	}
}

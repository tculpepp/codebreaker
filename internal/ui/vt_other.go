//go:build !windows

package ui

import "io"

// enableVT reports whether w can show ANSI escape codes. Terminals on
// Linux, macOS and other Unix systems always can, so there's nothing to
// turn on.
//
// Go note: the "//go:build !windows" line at the top of this file is a
// build constraint. The file is compiled on every system except Windows;
// vt_windows.go, with "//go:build windows", is compiled only on Windows.
// Both define enableVT, and exactly one of them is ever part of a build.
func enableVT(w io.Writer) bool { return true }

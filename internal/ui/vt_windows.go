//go:build windows

package ui

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// enableVT turns on ANSI escape-code support ("virtual terminal
// processing") for the Windows console behind w, and reports whether that
// worked. Windows Terminal has it on already; the older console needs it
// switched on. If it fails, the Renderer draws plain text instead.
func enableVT(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

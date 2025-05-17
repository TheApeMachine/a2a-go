package kubrick

import (
	"golang.org/x/term"
)

// GetTerminalSize returns the current terminal dimensions
func GetTerminalSize(fd int) (width int, height int, err error) {
	return term.GetSize(fd)
}

// MakeRawMode puts the terminal in raw mode
func MakeRawMode(fd int) (interface{}, error) {
	return term.MakeRaw(fd)
}

// RestoreMode restores the terminal to its original state
func RestoreMode(fd int, state interface{}) error {
	if termState, ok := state.(*term.State); ok {
		return term.Restore(fd, termState)
	}
	return nil
}
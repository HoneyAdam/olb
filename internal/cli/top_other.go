//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package cli

import (
	"fmt"
)

func newTerminal() (*Terminal, error) {
	return nil, fmt.Errorf("TUI not supported on this platform")
}

func (t *Terminal) restore() error {
	return nil
}

func getTerminalSizePlatform() (int, int) {
	return 80, 24
}

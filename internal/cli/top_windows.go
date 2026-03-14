//go:build windows
// +build windows

package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

const (
	// Console input modes
	ENABLE_ECHO_INPUT             = 0x0004
	ENABLE_LINE_INPUT             = 0x0002
	ENABLE_PROCESSED_INPUT        = 0x0001
	ENABLE_WINDOW_INPUT           = 0x0008
	ENABLE_VIRTUAL_TERMINAL_INPUT = 0x0200

	// Console output modes
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	DISABLE_NEWLINE_AUTO_RETURN        = 0x0008
)

// consoleScreenBufferInfo holds console buffer information.
type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

// terminalState holds the original console mode.
type terminalState struct {
	originalInMode  uint32
	originalOutMode uint32
	inHandle        syscall.Handle
	outHandle       syscall.Handle
}

// newTerminal initializes the terminal for raw mode on Windows.
func newTerminal() (*Terminal, error) {
	inHandle := syscall.Handle(os.Stdin.Fd())
	outHandle := syscall.Handle(os.Stdout.Fd())

	// Get current console modes
	var originalInMode, originalOutMode uint32

	r, _, err := procGetConsoleMode.Call(uintptr(inHandle), uintptr(unsafe.Pointer(&originalInMode)))
	if r == 0 {
		return nil, fmt.Errorf("GetConsoleMode (input) failed: %v", err)
	}

	r, _, err = procGetConsoleMode.Call(uintptr(outHandle), uintptr(unsafe.Pointer(&originalOutMode)))
	if r == 0 {
		return nil, fmt.Errorf("GetConsoleMode (output) failed: %v", err)
	}

	// Save original state
	state := &terminalState{
		originalInMode:  originalInMode,
		originalOutMode: originalOutMode,
		inHandle:        inHandle,
		outHandle:       outHandle,
	}

	// Set new input mode (disable echo and line input)
	newInMode := originalInMode &^ (ENABLE_ECHO_INPUT | ENABLE_LINE_INPUT)
	newInMode |= ENABLE_VIRTUAL_TERMINAL_INPUT

	r, _, err = procSetConsoleMode.Call(uintptr(inHandle), uintptr(newInMode))
	if r == 0 {
		// Try without VT input mode
		newInMode = originalInMode &^ (ENABLE_ECHO_INPUT | ENABLE_LINE_INPUT)
		r, _, err = procSetConsoleMode.Call(uintptr(inHandle), uintptr(newInMode))
		if r == 0 {
			return nil, fmt.Errorf("SetConsoleMode (input) failed: %v", err)
		}
	}

	// Enable VT processing on output
	newOutMode := originalOutMode | ENABLE_VIRTUAL_TERMINAL_PROCESSING | DISABLE_NEWLINE_AUTO_RETURN
	r, _, _ = procSetConsoleMode.Call(uintptr(outHandle), uintptr(newOutMode))
	// Ignore error - VT processing might not be available

	return &Terminal{
		originalState: state,
	}, nil
}

// restore restores the terminal to its original state on Windows.
func (t *Terminal) restore() error {
	state, ok := t.originalState.(*terminalState)
	if !ok {
		return fmt.Errorf("invalid terminal state")
	}

	// Restore original modes
	procSetConsoleMode.Call(uintptr(state.inHandle), uintptr(state.originalInMode))
	procSetConsoleMode.Call(uintptr(state.outHandle), uintptr(state.originalOutMode))

	// Clear screen and show cursor
	fmt.Print("\x1b[2J\x1b[H\x1b[?25h")
	return nil
}

// getTerminalSizePlatform returns the terminal size on Windows.
func getTerminalSizePlatform() (int, int) {
	outHandle := syscall.Handle(os.Stdout.Fd())

	var info consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(uintptr(outHandle), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 80, 24 // Default size
	}

	width := int(info.Window.Right - info.Window.Left + 1)
	height := int(info.Window.Bottom - info.Window.Top + 1)

	return width, height
}

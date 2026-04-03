//go:build windows

package logging

// ReopenHandler is a no-op on Windows (SIGUSR1 not available).
type ReopenHandler struct{}

// NewReopenHandler creates a no-op handler on Windows.
func NewReopenHandler() *ReopenHandler {
	return &ReopenHandler{}
}

// AddOutput is a no-op on Windows.
func (h *ReopenHandler) AddOutput(out *RotatingFileOutput) {}

// Start is a no-op on Windows.
func (h *ReopenHandler) Start() {}

// Stop is a no-op on Windows.
func (h *ReopenHandler) Stop() {}

// EnableLogReopen is a no-op on Windows.
func EnableLogReopen(outputs ...*RotatingFileOutput) {
	// SIGUSR1 not available on Windows
}

// StopLogReopen is a no-op on Windows.
func StopLogReopen() {
	// No-op on Windows
}

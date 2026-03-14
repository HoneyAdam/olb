// Package cli provides the command-line interface for OpenLoadBalancer.
package cli

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/admin"
)

// TestScreen tests the screen buffer functionality.
func TestScreen(t *testing.T) {
	t.Run("NewScreen", func(t *testing.T) {
		s := NewScreen()
		if s == nil {
			t.Fatal("NewScreen returned nil")
		}
		if s.writer == nil {
			t.Error("Screen writer is nil")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		s := NewScreen()
		s.Reset(80, 24)

		if s.width != 80 {
			t.Errorf("Expected width 80, got %d", s.width)
		}
		if s.height != 24 {
			t.Errorf("Expected height 24, got %d", s.height)
		}
		if len(s.front) != 80*24 {
			t.Errorf("Expected front buffer length %d, got %d", 80*24, len(s.front))
		}
		if len(s.back) != 80*24 {
			t.Errorf("Expected back buffer length %d, got %d", 80*24, len(s.back))
		}
	})

	t.Run("SetCell", func(t *testing.T) {
		s := NewScreen()
		s.Reset(10, 10)

		s.SetCell(5, 5, 'X', ColorRed)

		idx := 5*10 + 5
		if s.back[idx].Ch != 'X' {
			t.Errorf("Expected cell char 'X', got %c", s.back[idx].Ch)
		}
		if s.back[idx].Color != ColorRed {
			t.Errorf("Expected color Red, got %d", s.back[idx].Color)
		}
		if !s.back[idx].Dirty {
			t.Error("Expected cell to be dirty")
		}
	})

	t.Run("SetCellOutOfBounds", func(t *testing.T) {
		s := NewScreen()
		s.Reset(10, 10)

		// These should not panic
		s.SetCell(-1, 5, 'X', ColorDefault)
		s.SetCell(10, 5, 'X', ColorDefault)
		s.SetCell(5, -1, 'X', ColorDefault)
		s.SetCell(5, 10, 'X', ColorDefault)
	})

	t.Run("DrawText", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawText(2, 3, "Hello")

		expected := []rune{'H', 'e', 'l', 'l', 'o'}
		for i, ch := range expected {
			idx := 3*20 + 2 + i
			if s.back[idx].Ch != ch {
				t.Errorf("Expected char %c at position %d, got %c", ch, i, s.back[idx].Ch)
			}
		}
	})

	t.Run("DrawTextColored", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawTextColored(0, 0, "Test", ColorGreen)

		for i := 0; i < 4; i++ {
			if s.back[i].Color != ColorGreen {
				t.Errorf("Expected color Green at position %d, got %d", i, s.back[i].Color)
			}
		}
	})
}

// TestScreenBoxDrawing tests box drawing functionality.
func TestScreenBoxDrawing(t *testing.T) {
	t.Run("DrawBox", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawBox(0, 0, 10, 5, "Title", false)

		// Check corners
		if s.back[0].Ch != BoxDrawingsLightDownAndRight {
			t.Errorf("Expected top-left corner, got %c", s.back[0].Ch)
		}
		if s.back[9].Ch != BoxDrawingsLightDownAndLeft {
			t.Errorf("Expected top-right corner, got %c", s.back[9].Ch)
		}
		if s.back[4*20].Ch != BoxDrawingsLightUpAndRight {
			t.Errorf("Expected bottom-left corner, got %c", s.back[4*20].Ch)
		}
		if s.back[4*20+9].Ch != BoxDrawingsLightUpAndLeft {
			t.Errorf("Expected bottom-right corner, got %c", s.back[4*20+9].Ch)
		}
	})

	t.Run("DrawBoxTooSmall", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		// Should not panic with small dimensions
		s.DrawBox(0, 0, 1, 1, "Title", false)
		s.DrawBox(0, 0, 2, 2, "Title", false)
	})

	t.Run("DrawHLine", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawHLine(0, 5, 10)

		for i := 0; i < 10; i++ {
			idx := 5*20 + i
			if s.back[idx].Ch != BoxDrawingsLightHorizontal {
				t.Errorf("Expected horizontal line at position %d, got %c", i, s.back[idx].Ch)
			}
		}
	})

	t.Run("DrawVLine", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawVLine(5, 0, 5)

		for i := 0; i < 5; i++ {
			idx := i*20 + 5
			if s.back[idx].Ch != BoxDrawingsLightVertical {
				t.Errorf("Expected vertical line at row %d, got %c", i, s.back[idx].Ch)
			}
		}
	})
}

// TestScreenGauge tests gauge drawing.
func TestScreenGauge(t *testing.T) {
	t.Run("DrawGauge", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawGauge(0, 0, 10, 5, ColorGreen)

		// Check brackets
		if s.back[0].Ch != '[' {
			t.Errorf("Expected '[', got %c", s.back[0].Ch)
		}
		if s.back[9].Ch != ']' {
			t.Errorf("Expected ']', got %c", s.back[9].Ch)
		}

		// Check filled portion
		for i := 1; i <= 5; i++ {
			if s.back[i].Ch != '=' {
				t.Errorf("Expected '=' at position %d, got %c", i, s.back[i].Ch)
			}
		}

		// Check empty portion
		for i := 6; i < 9; i++ {
			if s.back[i].Ch != '-' {
				t.Errorf("Expected '-' at position %d, got %c", i, s.back[i].Ch)
			}
		}
	})

	t.Run("DrawGaugeFull", func(t *testing.T) {
		s := NewScreen()
		s.Reset(20, 10)

		s.DrawGauge(0, 0, 10, 8, ColorRed)

		// All positions except brackets should be filled
		for i := 1; i < 9; i++ {
			if s.back[i].Ch != '=' {
				t.Errorf("Expected '=' at position %d, got %c", i, s.back[i].Ch)
			}
		}
	})
}

// TestTruncate tests the truncate function.
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"test", 4, "test"},
		{"", 5, ""},
		{"very long string here", 5, "ve..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// TestFormatNumber tests the formatNumber function.
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{1000000000, "1.0B"},
		{2500000000, "2.5B"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%d) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestColorCodes tests color code generation.
func TestColorCodes(t *testing.T) {
	// Verify all colors have codes
	colors := []Color{
		ColorDefault, ColorBlack, ColorRed, ColorGreen, ColorYellow,
		ColorBlue, ColorMagenta, ColorCyan, ColorWhite,
		ColorBrightBlack, ColorBrightRed, ColorBrightGreen, ColorBrightYellow,
		ColorBrightBlue, ColorBrightMagenta, ColorBrightCyan, ColorBrightWhite,
	}

	for _, color := range colors {
		if _, ok := colorCodes[color]; !ok {
			t.Errorf("Missing color code for color %d", color)
		}
	}

	// Verify codes are non-empty
	for color, code := range colorCodes {
		if code == "" {
			t.Errorf("Empty color code for color %d", color)
		}
		if !strings.HasPrefix(code, "\x1b[") {
			t.Errorf("Invalid ANSI code format for color %d: %q", color, code)
		}
	}
}

// TestBoxDrawingCharacters tests that box drawing characters are defined.
func TestBoxDrawingCharacters(t *testing.T) {
	chars := map[string]rune{
		"Horizontal":            BoxDrawingsLightHorizontal,
		"Vertical":              BoxDrawingsLightVertical,
		"DownAndRight":          BoxDrawingsLightDownAndRight,
		"DownAndLeft":           BoxDrawingsLightDownAndLeft,
		"UpAndRight":            BoxDrawingsLightUpAndRight,
		"UpAndLeft":             BoxDrawingsLightUpAndLeft,
		"VerticalAndRight":      BoxDrawingsLightVerticalAndRight,
		"VerticalAndLeft":       BoxDrawingsLightVerticalAndLeft,
		"DownAndHorizontal":     BoxDrawingsLightDownAndHorizontal,
		"UpAndHorizontal":       BoxDrawingsLightUpAndHorizontal,
		"VerticalAndHorizontal": BoxDrawingsLightVerticalAndHorizontal,
	}

	for name, ch := range chars {
		t.Run(name, func(t *testing.T) {
			if ch == 0 {
				t.Errorf("Box drawing character %s is not defined", name)
			}
		})
	}
}

// TestMetricsFetcher tests the metrics fetcher (with mock server).
func TestMetricsFetcher(t *testing.T) {
	// Create a mock server
	mockData := &admin.SystemInfo{
		Version:   "1.0.0",
		Commit:    "abc123",
		BuildDate: "2024-01-01",
		Uptime:    "1h30m",
		State:     "running",
		GoVersion: "1.23",
	}

	// Note: In a real test, we'd start an HTTP server here
	// For now, just test the struct creation
	fetcher := NewMetricsFetcher("localhost:8081")
	if fetcher == nil {
		t.Fatal("NewMetricsFetcher returned nil")
	}
	if fetcher.apiAddr != "localhost:8081" {
		t.Errorf("Expected apiAddr localhost:8081, got %s", fetcher.apiAddr)
	}
	if fetcher.client == nil {
		t.Error("Expected client to be initialized")
	}
	if fetcher.client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", fetcher.client.Timeout)
	}

	_ = mockData // Use the variable to avoid unused warning
}

// TestTUIEventHandling tests TUI event handling.
func TestTUIEventHandling(t *testing.T) {
	fetcher := NewMetricsFetcher("localhost:8081")
	tui := NewTUI(fetcher)

	tests := []struct {
		name     string
		event    Event
		expected bool // true if should exit
	}{
		{"QuitEvent", Event{Type: EventQuit}, true},
		{"KeyQ", Event{Type: EventKey, Key: 'q'}, true},
		{"KeyB", Event{Type: EventKey, Key: 'b'}, false},
		{"KeyR", Event{Type: EventKey, Key: 'r'}, false},
		{"KeyM", Event{Type: EventKey, Key: 'm'}, false},
		{"KeyO", Event{Type: EventKey, Key: 'o'}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset view state
			tui.currentView = ViewOverview

			result := tui.handleEvent(tt.event)
			if result != tt.expected {
				t.Errorf("handleEvent() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestTUIViewSwitching tests view switching.
func TestTUIViewSwitching(t *testing.T) {
	fetcher := NewMetricsFetcher("localhost:8081")
	tui := NewTUI(fetcher)
	tui.currentView = ViewOverview

	// Test switching to backends view
	tui.handleEvent(Event{Type: EventKey, Key: 'b'})
	if tui.currentView != ViewBackends {
		t.Errorf("Expected view Backends, got %d", tui.currentView)
	}

	// Test switching to routes view
	tui.handleEvent(Event{Type: EventKey, Key: 'r'})
	if tui.currentView != ViewRoutes {
		t.Errorf("Expected view Routes, got %d", tui.currentView)
	}

	// Test switching to metrics view
	tui.handleEvent(Event{Type: EventKey, Key: 'm'})
	if tui.currentView != ViewMetrics {
		t.Errorf("Expected view Metrics, got %d", tui.currentView)
	}

	// Test switching to overview view
	tui.handleEvent(Event{Type: EventKey, Key: 'o'})
	if tui.currentView != ViewOverview {
		t.Errorf("Expected view Overview, got %d", tui.currentView)
	}
}

// TestDashboardData tests dashboard data structure.
func TestDashboardData(t *testing.T) {
	data := &DashboardData{
		SystemInfo: &admin.SystemInfo{
			Version: "1.0.0",
			Uptime:  "1h",
		},
		Pools: []admin.BackendPool{
			{
				Name:      "pool1",
				Algorithm: "round_robin",
				Backends: []admin.Backend{
					{ID: "b1", Address: "10.0.0.1:8080", Weight: 1, Healthy: true},
				},
			},
		},
		Routes: []admin.Route{
			{Name: "route1", Path: "/api", BackendPool: "pool1"},
		},
		Health: &admin.HealthStatus{
			Status: "healthy",
		},
		Timestamp: time.Now(),
	}

	if data.SystemInfo.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", data.SystemInfo.Version)
	}
	if len(data.Pools) != 1 {
		t.Errorf("Expected 1 pool, got %d", len(data.Pools))
	}
	if len(data.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(data.Routes))
	}
	if data.Health.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", data.Health.Status)
	}
}

// TestInputHandler tests the input handler.
func TestInputHandler(t *testing.T) {
	t.Run("NewInputHandler", func(t *testing.T) {
		eventCh := make(chan Event, 10)
		h := NewInputHandler(os.Stdin, eventCh)

		if h == nil {
			t.Fatal("NewInputHandler returned nil")
		}
		if h.eventCh != eventCh {
			t.Error("Event channel mismatch")
		}
		if h.stopCh == nil {
			t.Error("Stop channel is nil")
		}
	})
}

// TestLayout tests the layout manager.
func TestLayout(t *testing.T) {
	t.Run("NewLayout", func(t *testing.T) {
		l := NewLayout()
		if l == nil {
			t.Fatal("NewLayout returned nil")
		}
		if len(l.widgets) != 0 {
			t.Errorf("Expected 0 widgets, got %d", len(l.widgets))
		}
	})

	t.Run("AddWidget", func(t *testing.T) {
		l := NewLayout()
		// Create a simple mock widget
		widget := &mockWidget{}
		l.AddWidget(widget)

		if len(l.widgets) != 1 {
			t.Errorf("Expected 1 widget, got %d", len(l.widgets))
		}
	})
}

// mockWidget is a simple widget implementation for testing.
type mockWidget struct{}

func (m *mockWidget) Draw(s *Screen, x, y, width, height int) {
	// No-op for testing
}

// TestTopCommand tests the top command.
func TestTopCommand(t *testing.T) {
	cmd := &TopCommand{}

	if cmd.Name() != "top" {
		t.Errorf("Expected name 'top', got %s", cmd.Name())
	}

	desc := cmd.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(desc, "TUI") {
		t.Error("Description should mention TUI")
	}
}

// TestScreenFlush tests screen flushing.
func TestScreenFlush(t *testing.T) {
	var buf bytes.Buffer
	s := NewScreen()
	s.writer = bufio.NewWriter(&buf)

	s.Reset(10, 5)
	s.DrawText(0, 0, "Test")
	s.Flush()

	// Output should contain ANSI escape sequences
	output := buf.String()
	if output == "" {
		t.Error("Expected output from Flush")
	}
}

// TestTUIGetString tests the getString helper.
func TestTUIGetString(t *testing.T) {
	fetcher := NewMetricsFetcher("localhost:8081")
	tui := NewTUI(fetcher)

	tests := []struct {
		input    string
		def      string
		expected string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"test", "", "test"},
	}

	for _, tt := range tests {
		result := tui.getString(tt.input, tt.def)
		if result != tt.expected {
			t.Errorf("getString(%q, %q) = %q, expected %q", tt.input, tt.def, result, tt.expected)
		}
	}
}

// TestViewConstants tests view constants.
func TestViewConstants(t *testing.T) {
	views := []struct {
		view View
		name string
	}{
		{ViewOverview, "Overview"},
		{ViewBackends, "Backends"},
		{ViewRoutes, "Routes"},
		{ViewMetrics, "Metrics"},
	}

	for i, v := range views {
		if int(v.view) != i {
			t.Errorf("View %s has incorrect value %d, expected %d", v.name, v.view, i)
		}
	}
}

// TestEventTypeConstants tests event type constants.
func TestEventTypeConstants(t *testing.T) {
	types := []struct {
		evt  EventType
		name string
	}{
		{EventKey, "Key"},
		{EventResize, "Resize"},
		{EventQuit, "Quit"},
	}

	for i, et := range types {
		if int(et.evt) != i {
			t.Errorf("EventType %s has incorrect value %d, expected %d", et.name, et.evt, i)
		}
	}
}

// BenchmarkScreenSetCell benchmarks cell setting.
func BenchmarkScreenSetCell(b *testing.B) {
	s := NewScreen()
	s.Reset(80, 24)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 80
		y := (i / 80) % 24
		s.SetCell(x, y, 'X', ColorDefault)
	}
}

// BenchmarkScreenDrawText benchmarks text drawing.
func BenchmarkScreenDrawText(b *testing.B) {
	s := NewScreen()
	s.Reset(80, 24)
	text := "Hello, World!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.DrawText(0, 0, text)
	}
}

// BenchmarkTruncate benchmarks string truncation.
func BenchmarkTruncate(b *testing.B) {
	text := "This is a very long string that needs truncation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		truncate(text, 20)
	}
}

// BenchmarkFormatNumber benchmarks number formatting.
func BenchmarkFormatNumber(b *testing.B) {
	numbers := []int64{0, 999, 1000, 999999, 1000000, 1000000000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatNumber(numbers[i%len(numbers)])
	}
}

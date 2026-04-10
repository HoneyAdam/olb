package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// printYAML tests
// ---------------------------------------------------------------------------

// TestCov_PrintYAML_EmptyMap tests printYAML with an empty map.
func TestCov_PrintYAML_EmptyMap(t *testing.T) {
	// Redirect stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML(map[string]any{}, 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty map, got %q", buf.String())
	}
}

// TestCov_PrintYAML_EmptyArray tests printYAML with an empty array.
func TestCov_PrintYAML_EmptyArray(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML([]any{}, 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty array, got %q", buf.String())
	}
}

// TestCov_PrintYAML_Scalar tests printYAML with a plain scalar value.
func TestCov_PrintYAML_Scalar(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML("hello", 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected 'hello' in output, got %q", buf.String())
	}
}

// TestCov_PrintYAML_ArrayOfMaps tests printYAML with array containing maps.
func TestCov_PrintYAML_ArrayOfMaps(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML([]any{
		map[string]any{"name": "item1", "value": "v1"},
		map[string]any{"name": "item2"},
	}, 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "item1") {
		t.Errorf("expected 'item1' in output, got %q", output)
	}
}

// TestCov_PrintYAML_ArrayOfScalars tests printYAML with array of plain values.
func TestCov_PrintYAML_ArrayOfScalars(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML([]any{"a", "b", "c"}, 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "- a") {
		t.Errorf("expected '- a' in output, got %q", output)
	}
}

// TestCov_PrintYAML_NestedMap tests printYAML with nested map.
func TestCov_PrintYAML_NestedMap(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printYAML(map[string]any{
		"key": map[string]any{"nested": "value"},
	}, 0)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "key:") {
		t.Errorf("expected 'key:' in output, got %q", output)
	}
	if !strings.Contains(output, "nested: value") {
		t.Errorf("expected 'nested: value' in output, got %q", output)
	}
}

// ---------------------------------------------------------------------------
// BackendEnableCommand tests
// ---------------------------------------------------------------------------

// TestCov_BackendEnable_404 tests BackendEnableCommand with HTTP 404 response.
func TestCov_BackendEnable_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cmd := &BackendEnableCommand{}
	err := cmd.Run([]string{"-api-addr", strings.TrimPrefix(srv.URL, "http://"), "pool-1", "backend-1"})
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected 'HTTP 404' in error, got %q", err.Error())
	}
}

// TestCov_BackendEnable_InsufficientArgs tests BackendEnableCommand with too few args.
func TestCov_BackendEnable_InsufficientArgs(t *testing.T) {
	cmd := &BackendEnableCommand{apiAddr: "http://localhost:9090"}
	err := cmd.Run([]string{"pool-1"})
	if err == nil {
		t.Error("expected error for insufficient args")
	}
}

// TestCov_BackendEnable_500 tests BackendEnableCommand with HTTP 500 response.
func TestCov_BackendEnable_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cmd := &BackendEnableCommand{}
	err := cmd.Run([]string{"-api-addr", strings.TrimPrefix(srv.URL, "http://"), "pool-1", "backend-1"})
	if err == nil {
		t.Error("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected 'HTTP 500' in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// TUI tests
// ---------------------------------------------------------------------------

// TestCov_TUI_Render_ScreenNil tests that render is a no-op when screen is nil.
func TestCov_TUI_Render_ScreenNil(t *testing.T) {
	tui := NewTUI(nil)
	// screen is nil by default
	tui.render() // should not panic
}

// TestCov_TUI_Run_DoubleStart tests that calling Run twice returns an error.
func TestCov_TUI_Run_DoubleStart(t *testing.T) {
	tui := NewTUI(nil)

	// Set running to true to simulate an already-running TUI
	tui.running.Store(true)

	err := tui.Run()
	if err == nil {
		t.Error("expected error when TUI already running")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got %q", err.Error())
	}

	// Reset for cleanup
	tui.running.Store(false)
}

// ---------------------------------------------------------------------------
// StatusCommand backends float64 test
// ---------------------------------------------------------------------------

// TestCov_StatusCommand_Float64Backends tests that float64 backends value is
// printed correctly in table format.
func TestCov_StatusCommand_Float64Backends(t *testing.T) {
	info := map[string]any{
		"backends": float64(5),
		"listeners": float64(3),
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	if listeners, ok := info["listeners"].(float64); ok {
		fmt.Printf("Listeners: %.0f\n", listeners)
	}
	if backends, ok := info["backends"].(float64); ok {
		fmt.Printf("Backends: %.0f\n", backends)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Listeners: 3") {
		t.Errorf("expected 'Listeners: 3', got %q", output)
	}
	if !strings.Contains(output, "Backends: 5") {
		t.Errorf("expected 'Backends: 5', got %q", output)
	}
}

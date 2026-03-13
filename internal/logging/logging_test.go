package logging

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{TraceLevel, "TRACE"},
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{FatalLevel, "FATAL"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.expected {
			t.Errorf("Level(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"trace", TraceLevel},
		{"DEBUG", DebugLevel},
		{"Info", InfoLevel},
		{"WARN", WarnLevel},
		{"warning", WarnLevel},
		{"ERROR", ErrorLevel},
		{"FATAL", FatalLevel},
		{"SILENT", SilentLevel},
		{"unknown", InfoLevel},
		{"", InfoLevel},
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestField_Creators(t *testing.T) {
	f1 := String("key", "value")
	if f1.Key != "key" || f1.Value != "value" {
		t.Error("String field incorrect")
	}

	f2 := Int("count", 42)
	if f2.Key != "count" || f2.Value != 42 {
		t.Error("Int field incorrect")
	}

	f3 := Int64("big", 9223372036854775807)
	if f3.Key != "big" || f3.Value != int64(9223372036854775807) {
		t.Error("Int64 field incorrect")
	}

	f4 := Uint64("pos", 18446744073709551615)
	if f4.Key != "pos" || f4.Value != uint64(18446744073709551615) {
		t.Error("Uint64 field incorrect")
	}

	f5 := Float64("pi", 3.14)
	if f5.Key != "pi" || f5.Value != 3.14 {
		t.Error("Float64 field incorrect")
	}

	f6 := Bool("flag", true)
	if f6.Key != "flag" || f6.Value != true {
		t.Error("Bool field incorrect")
	}

	f7 := Error(errors.New("test"))
	if f7.Key != "error" {
		t.Error("Error field key incorrect")
	}

	f8 := Duration("elapsed", time.Second)
	if f8.Key != "elapsed" || f8.Value != time.Second {
		t.Error("Duration field incorrect")
	}

	f9 := Any("any", []int{1, 2, 3})
	if f9.Key != "any" {
		t.Error("Any field key incorrect")
	}
}

func TestLogger_Basic(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))
	logger.SetLevel(DebugLevel)

	logger.Debug("debug message", String("key", "value"))
	logger.Info("info message", Int("count", 42))
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	if !strings.Contains(output, "debug message") {
		t.Error("Debug message not logged")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message not logged")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Field not logged")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Int field not logged")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))
	logger.SetLevel(WarnLevel)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	output := buf.String()

	if strings.Contains(output, "debug") {
		t.Error("Debug should be filtered")
	}
	if strings.Contains(output, "info") {
		t.Error("Info should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("Warn should not be filtered")
	}
	if !strings.Contains(output, "error") {
		t.Error("Error should not be filtered")
	}
}

func TestLogger_Enabled(t *testing.T) {
	logger := NewWithDefaults()
	logger.SetLevel(InfoLevel)

	if logger.Enabled(DebugLevel) {
		t.Error("Debug should not be enabled")
	}
	if !logger.Enabled(InfoLevel) {
		t.Error("Info should be enabled")
	}
	if !logger.Enabled(ErrorLevel) {
		t.Error("Error should be enabled")
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))

	child := logger.With(String("parent", "value"))
	child.Info("message", String("child", "data"))

	output := buf.String()

	if !strings.Contains(output, "parent=value") {
		t.Error("Parent field not inherited")
	}
	if !strings.Contains(output, "child=data") {
		t.Error("Child field not logged")
	}
}

func TestLogger_WithName(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))

	child := logger.WithName("MyLogger")
	child.Info("message")

	output := buf.String()

	if !strings.Contains(output, "logger=MyLogger") {
		t.Error("Logger name not logged")
	}
}

func TestLogger_Formatted(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))
	logger.SetLevel(DebugLevel)

	logger.Debugf("debug %s %d", "test", 42)
	logger.Infof("info %s", "message")

	output := buf.String()

	if !strings.Contains(output, "debug test 42") {
		t.Error("Debugf not working")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Infof not working")
	}
}

func TestLogger_FormattedLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))
	logger.SetLevel(WarnLevel)

	// These should be filtered (note: Sprintf is still called, just not logged)
	logger.Debugf("debug %s", "test")
	logger.Infof("info %s", "test")

	// These should be logged
	logger.Warnf("warn %s", "test")
	logger.Errorf("error %s", "test")

	output := buf.String()
	if strings.Contains(output, "debug") {
		t.Error("Debugf output should be filtered")
	}
	if strings.Contains(output, "info") {
		t.Error("Infof output should be filtered")
	}
	if !strings.Contains(output, "warn") || !strings.Contains(output, "error") {
		t.Error("Warnf/Errorf should be logged")
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	out := NewJSONOutput(&buf)

	out.Write(InfoLevel, "test message", []Field{
		String("key", "value"),
		Int("count", 42),
		Bool("flag", true),
	})

	output := buf.String()

	// Check JSON structure
	if !strings.HasPrefix(output, "{") {
		t.Error("JSON should start with {")
	}
	if !strings.HasSuffix(output, "}\n") {
		t.Error("JSON should end with }\n")
	}
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Error("Level not in JSON")
	}
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Error("Message not in JSON")
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Error("String field not in JSON")
	}
	if !strings.Contains(output, `"count":42`) {
		t.Error("Int field not in JSON")
	}
	if !strings.Contains(output, `"flag":true`) {
		t.Error("Bool field not in JSON")
	}
}

func TestJSONOutput_Escaping(t *testing.T) {
	var buf bytes.Buffer
	out := NewJSONOutput(&buf)

	out.Write(InfoLevel, `test "quoted" message`, []Field{
		String("path", "C:\\Users\\test"),
		String("newline", "line1\nline2"),
		String("tab", "col1\tcol2"),
	})

	output := buf.String()

	// Check escaping
	if strings.Contains(output, `"msg":"test "quoted" message"`) {
		t.Error("Quotes not escaped")
	}
	if strings.Contains(output, `C:\Users\test`) {
		t.Error("Backslashes not escaped")
	}
	if strings.Contains(output, "line1\nline2") {
		t.Error("Newline not escaped")
	}
}

func TestTextOutput(t *testing.T) {
	var buf bytes.Buffer
	out := NewTextOutput(&buf)

	out.Write(InfoLevel, "test message", []Field{
		String("key", "value"),
		Int("count", 42),
	})

	output := buf.String()

	if !strings.Contains(output, "INFO") {
		t.Error("Level not in output")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Message not in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Field not in output")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Int field not in output")
	}
}

func TestMultiOutput(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	out1 := NewTextOutput(&buf1)
	out2 := NewTextOutput(&buf2)

	multi := NewMultiOutput(out1, out2)
	multi.Write(InfoLevel, "test", nil)

	if !strings.Contains(buf1.String(), "test") {
		t.Error("First output not written")
	}
	if !strings.Contains(buf2.String(), "test") {
		t.Error("Second output not written")
	}
}

func TestRotatingFileOutput(t *testing.T) {
	// Create temp file
	tmpFile := t.TempDir() + "/test.log"

	opts := RotatingFileOptions{
		Filename:   tmpFile,
		MaxSize:    100, // Small for testing
		MaxBackups: 3,
		Compress:   false,
	}

	out, err := NewRotatingFileOutput(opts)
	if err != nil {
		t.Fatalf("Failed to create rotating file: %v", err)
	}
	defer out.Close()

	// Write some data
	for i := 0; i < 10; i++ {
		out.Write(InfoLevel, "test message with some padding to exceed size", nil)
	}

	// Check file exists
	if _, err := os.Stat(tmpFile); err != nil {
		t.Error("Log file not created")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"simple", "simple"},
		{"with space", `"with space"`},
		{42, "42"},
		{errors.New("err"), "err"},
		{time.Second, "1s"},
	}

	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.expected {
			t.Errorf("formatValue(%v) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	var buf bytes.Buffer
	logger := New(NewTextOutput(&buf))
	logger.SetLevel(InfoLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", String("key", "value"), Int("count", i))
	}
}

func BenchmarkJSONOutput_Write(b *testing.B) {
	var buf bytes.Buffer
	out := NewJSONOutput(&buf)
	fields := []Field{
		String("key", "value"),
		Int("count", 42),
		Bool("flag", true),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		out.Write(InfoLevel, "test message", fields)
	}
}

func BenchmarkTextOutput_Write(b *testing.B) {
	var buf bytes.Buffer
	out := NewTextOutput(&buf)
	fields := []Field{
		String("key", "value"),
		Int("count", 42),
		Bool("flag", true),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		out.Write(InfoLevel, "test message", fields)
	}
}

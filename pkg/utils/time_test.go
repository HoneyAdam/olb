package utils

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1h30m", time.Hour + 30*time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"2d12h", 2*24*time.Hour + 12*time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"100ms", 100 * time.Millisecond, false},
		{"1.5h", time.Hour + 30*time.Minute, false},
		{"", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDuration(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("ParseDuration(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestMustParseDuration(t *testing.T) {
	// Valid
	d := MustParseDuration("1h")
	if d != time.Hour {
		t.Errorf("MustParseDuration(\"1h\") = %v, want %v", d, time.Hour)
	}

	// Invalid should panic
	panicCalled := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicCalled = true
			}
		}()
		MustParseDuration("invalid")
	}()
	if !panicCalled {
		t.Error("MustParseDuration should panic on invalid input")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0s"},
		{time.Hour, "1h"},
		{time.Hour + time.Minute, "1h1m"},
		{time.Hour + 30*time.Minute + 5*time.Second, "1h30m5s"},
		{24 * time.Hour, "1d"},
		{7 * 24 * time.Hour, "1w"},
		{-time.Hour, "-1h"},
		{100 * time.Millisecond, "0.100s"},
		{time.Second, "1s"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, want %s", tt.d, got, tt.expected)
		}
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"100", 100, false},
		{"1KB", 1000, false},
		{"1KiB", 1024, false},
		{"1MB", 1000 * 1000, false},
		{"1MiB", 1024 * 1024, false},
		{"1GB", 1000 * 1000 * 1000, false},
		{"1GiB", 1024 * 1024 * 1024, false},
		{"1TB", 1000000000000, false},
		{"1.5MB", 1500000, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"MB", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseByteSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseByteSize(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("ParseByteSize(%s) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestMustParseByteSize(t *testing.T) {
	n := MustParseByteSize("1MB")
	if n != 1000000 {
		t.Errorf("MustParseByteSize(\"1MB\") = %d, want 1000000", n)
	}
}

func TestFormatByteSize(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1 KiB"},
		{1536, "1.50 KiB"},
		{1024 * 1024, "1 MiB"},
		{1024 * 1024 * 1024, "1 GiB"},
		{1024 * 1024 * 1024 * 1024, "1 TiB"},
		{-1024, "-1 KiB"},
	}

	for _, tt := range tests {
		got := FormatByteSize(tt.n)
		if got != tt.expected {
			t.Errorf("FormatByteSize(%d) = %s, want %s", tt.n, got, tt.expected)
		}
	}
}

func TestTruncateDuration(t *testing.T) {
	d := time.Hour + 30*time.Minute + 45*time.Second
	truncated := TruncateDuration(d, time.Hour)
	if truncated != time.Hour {
		t.Errorf("TruncateDuration = %v, want %v", truncated, time.Hour)
	}

	truncated = TruncateDuration(d, time.Minute)
	if truncated != time.Hour+30*time.Minute {
		t.Errorf("TruncateDuration = %v, want %v", truncated, time.Hour+30*time.Minute)
	}
}

func TestRoundDuration(t *testing.T) {
	d := time.Hour + 30*time.Minute
	rounded := RoundDuration(d, time.Hour)
	if rounded != 2*time.Hour {
		t.Errorf("RoundDuration = %v, want %v", rounded, 2*time.Hour)
	}

	d = time.Hour + 15*time.Minute
	rounded = RoundDuration(d, time.Hour)
	if rounded != time.Hour {
		t.Errorf("RoundDuration = %v, want %v", rounded, time.Hour)
	}
}

package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestCode_String(t *testing.T) {
	tests := []struct {
		code     Code
		expected string
	}{
		{CodeUnknown, "UNKNOWN"},
		{CodeInternal, "INTERNAL"},
		{CodeNotFound, "NOT_FOUND"},
		{CodeBackendNotFound, "BACKEND_NOT_FOUND"},
		{CodeConnectionRefused, "CONNECTION_REFUSED"},
		{CodeRateLimitExceeded, "RATE_LIMIT_EXCEEDED"},
		{Code(99999), "CODE_99999"}, // Unknown code
	}

	for _, tt := range tests {
		got := tt.code.String()
		if got != tt.expected {
			t.Errorf("Code(%d).String() = %s, want %s", tt.code, got, tt.expected)
		}
	}
}

func TestNew(t *testing.T) {
	err := New(CodeNotFound, "resource not found")

	if err.Code != CodeNotFound {
		t.Errorf("Code = %d, want %d", err.Code, CodeNotFound)
	}

	if err.Message != "resource not found" {
		t.Errorf("Message = %s, want 'resource not found'", err.Message)
	}

	if err.Cause != nil {
		t.Error("Cause should be nil for new error")
	}

	expected := "NOT_FOUND: resource not found"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeInvalidArg, "invalid value: %d", 42)

	expected := "INVALID_ARGUMENT: invalid value: 42"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(cause, CodeInternal, "operation failed")

	if err.Cause != cause {
		t.Error("Cause should be set")
	}

	expected := "INTERNAL: operation failed: underlying error"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// Wrap nil returns nil
	if Wrap(nil, CodeInternal, "test") != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrapf(t *testing.T) {
	cause := errors.New("connection refused")
	err := Wrapf(cause, CodeConnectionRefused, "backend %s failed", "backend1")

	expected := "CONNECTION_REFUSED: backend backend1 failed: connection refused"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestError_WithContext(t *testing.T) {
	err := New(CodeNotFound, "not found").
		WithContext("key", "value").
		WithContext("id", 123)

	if err.Context == nil {
		t.Fatal("Context should be set")
	}

	if err.Context["key"] != "value" {
		t.Errorf("Context['key'] = %v, want 'value'", err.Context["key"])
	}

	if err.Context["id"] != 123 {
		t.Errorf("Context['id'] = %v, want 123", err.Context["id"])
	}
}

func TestError_WithContextMap(t *testing.T) {
	err := New(CodeInternal, "error").
		WithContextMap(map[string]interface{}{
			"a": 1,
			"b": 2,
		})

	if err.Context["a"] != 1 || err.Context["b"] != 2 {
		t.Error("ContextMap not applied correctly")
	}
}

func TestError_Is(t *testing.T) {
	err1 := New(CodeNotFound, "not found")
	err2 := New(CodeNotFound, "also not found")
	err3 := New(CodeInternal, "internal")

	// Same code matches
	if !err1.Is(err2) {
		t.Error("Same code should match")
	}

	// Different code doesn't match
	if err1.Is(err3) {
		t.Error("Different code should not match")
	}

	// Wrapped error matches
	wrapped := Wrap(err1, CodeInternal, "wrapped")
	if !wrapped.Is(err1) {
		t.Error("Wrapped error should match cause by code")
	}

	// Check against sentinel error
	if !err1.Is(ErrNotFound) {
		t.Error("Should match sentinel error with same code")
	}
}

func TestIs(t *testing.T) {
	// Basic equality
	err1 := New(CodeNotFound, "test")
	if !Is(err1, err1) {
		t.Error("Is(err, err) should be true")
	}

	// Code matching
	if !Is(err1, ErrNotFound) {
		t.Error("Is should match by code")
	}

	// Wrapped error
	cause := errors.New("cause")
	wrapped := Wrap(cause, CodeInternal, "wrapped")
	if !Is(wrapped, cause) {
		t.Error("Is should find cause in chain")
	}

	// Nil handling
	if Is(nil, nil) {
		t.Error("Is(nil, nil) should be false")
	}

	if Is(err1, nil) {
		t.Error("Is(err, nil) should be false")
	}

	// Chain with standard errors
	stdErr := fmt.Errorf("wrapped: %w", ErrNotFound)
	if !Is(stdErr, ErrNotFound) {
		t.Error("Is should work with standard error wrapping")
	}
}

func TestAs(t *testing.T) {
	// Direct match
	original := New(CodeNotFound, "test")
	var target *Error
	if !As(original, &target) {
		t.Error("As should succeed for direct match")
	}
	if target != original {
		t.Error("As should set target to original")
	}

	// Wrapped match
	wrapped := Wrap(original, CodeInternal, "wrapped")
	target = nil
	if !As(wrapped, &target) {
		t.Error("As should succeed for wrapped error")
	}
	if target.Code != CodeInternal {
		t.Errorf("As should extract wrapped error, got code %d", target.Code)
	}

	// Standard error (no match)
	stdErr := errors.New("standard")
	target = nil
	if As(stdErr, &target) {
		t.Error("As should fail for standard error")
	}
}

func TestCodeOf(t *testing.T) {
	// Nil error
	if CodeOf(nil) != CodeUnknown {
		t.Errorf("CodeOf(nil) = %d, want CodeUnknown", CodeOf(nil))
	}

	// *Error
	err := New(CodeNotFound, "test")
	if CodeOf(err) != CodeNotFound {
		t.Errorf("CodeOf(err) = %d, want CodeNotFound", CodeOf(err))
	}

	// Standard error
	if CodeOf(errors.New("test")) != CodeUnknown {
		t.Error("CodeOf should return CodeUnknown for standard errors")
	}
}

func TestIsCode(t *testing.T) {
	err := New(CodeNotFound, "test")
	if !IsCode(err, CodeNotFound) {
		t.Error("IsCode should match")
	}
	if IsCode(err, CodeInternal) {
		t.Error("IsCode should not match different code")
	}
	if IsCode(nil, CodeNotFound) {
		t.Error("IsCode(nil) should be false")
	}
}

func TestWithContext(t *testing.T) {
	// With *Error
	err := New(CodeInternal, "error")
	result := WithContext(err, "key", "value")
	if e, ok := result.(*Error); !ok {
		t.Error("Result should be *Error")
	} else if e.Context["key"] != "value" {
		t.Error("Context not added")
	}

	// With nil
	if WithContext(nil, "key", "value") != nil {
		t.Error("WithContext(nil) should return nil")
	}

	// With standard error
	stdErr := errors.New("standard")
	if WithContext(stdErr, "key", "value") != stdErr {
		t.Error("WithContext should return as-is for non-*Error")
	}
}

func TestUnwrap(t *testing.T) {
	cause := errors.New("cause")
	wrapped := Wrap(cause, CodeInternal, "wrapped")

	if wrapped.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}

	// New error has no cause
	newErr := New(CodeNotFound, "new")
	if newErr.Unwrap() != nil {
		t.Error("New error should have nil Unwrap")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors have correct codes
	tests := []struct {
		err  *Error
		code Code
	}{
		{ErrUnknown, CodeUnknown},
		{ErrInternal, CodeInternal},
		{ErrNotFound, CodeNotFound},
		{ErrBackendNotFound, CodeBackendNotFound},
		{ErrPoolNotFound, CodePoolNotFound},
		{ErrRouteNotFound, CodeRouteNotFound},
		{ErrConnectionRefused, CodeConnectionRefused},
		{ErrTimeout, CodeTimeout},
		{ErrRateLimitExceeded, CodeRateLimitExceeded},
		{ErrCircuitOpen, CodeCircuitOpen},
	}

	for _, tt := range tests {
		if tt.err.Code != tt.code {
			t.Errorf("%v.Code = %d, want %d", tt.err, tt.err.Code, tt.code)
		}
	}
}

func TestErrorChain(t *testing.T) {
	// Build a chain: root -> wrap1 -> wrap2
	root := errors.New("root cause")
	wrap1 := Wrap(root, CodeConnectionRefused, "connection failed")
	wrap2 := Wrap(wrap1, CodeBackendUnavailable, "backend unavailable")

	// Can find root
	if !Is(wrap2, root) {
		t.Error("Should find root in chain")
	}

	// Can find wrap1
	if !Is(wrap2, wrap1) {
		t.Error("Should find wrap1 in chain")
	}

	// Can find by sentinel
	if !Is(wrap2, ErrConnectionRefused) {
		t.Error("Should find ErrConnectionRefused in chain")
	}

	// Error message shows chain
	msg := wrap2.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
}

func BenchmarkError_New(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(CodeNotFound, "not found")
	}
}

func BenchmarkError_Wrap(b *testing.B) {
	cause := errors.New("cause")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Wrap(cause, CodeInternal, "wrapped")
	}
}

func BenchmarkError_Is(b *testing.B) {
	err := New(CodeNotFound, "test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Is(err, ErrNotFound)
	}
}

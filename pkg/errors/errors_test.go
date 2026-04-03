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
		WithContextMap(map[string]any{
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

func TestErrorf(t *testing.T) {
	// Errorf is not directly implemented, but Newf serves the same purpose
	err := Newf(CodeInvalidArg, "invalid value: %s, expected: %s", "foo", "bar")

	expected := "INVALID_ARGUMENT: invalid value: foo, expected: bar"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// Test with integer formatting
	err2 := Newf(CodeInternal, "count: %d, rate: %.2f", 42, 3.14)
	expected2 := "INTERNAL: count: 42, rate: 3.14"
	if err2.Error() != expected2 {
		t.Errorf("Error() = %s, want %s", err2.Error(), expected2)
	}
}

func TestWrapWithNilError(t *testing.T) {
	// Wrap with nil should return nil
	result := Wrap(nil, CodeInternal, "this should not appear")
	if result != nil {
		t.Errorf("Wrap(nil) = %v, want nil", result)
	}

	// Wrapf with nil should also return nil
	result2 := Wrapf(nil, CodeNotFound, "formatted %s", "value")
	if result2 != nil {
		t.Errorf("Wrapf(nil) = %v, want nil", result2)
	}
}

func TestWrapfWithFormat(t *testing.T) {
	cause := errors.New("connection timeout")
	err := Wrapf(cause, CodeConnectionTimeout, "backend %s on port %d", "web01", 8080)

	expected := "CONNECTION_TIMEOUT: backend web01 on port 8080: connection timeout"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// Verify cause is preserved
	if err.Cause != cause {
		t.Error("Wrapf should preserve the cause")
	}

	// Verify code is set
	if err.Code != CodeConnectionTimeout {
		t.Errorf("Code = %d, want %d", err.Code, CodeConnectionTimeout)
	}
}

func TestIsWithWrappedErrors(t *testing.T) {
	// Create a chain: root -> wrap1 -> wrap2
	root := errors.New("root cause")
	wrap1 := Wrap(root, CodeConnectionRefused, "connection refused")
	wrap2 := Wrap(wrap1, CodeBackendUnavailable, "backend unavailable")

	// Should find root cause
	if !Is(wrap2, root) {
		t.Error("Is should find root cause in wrapped chain")
	}

	// Should find intermediate wrap
	if !Is(wrap2, wrap1) {
		t.Error("Is should find intermediate wrapped error")
	}

	// Should find by code match
	if !Is(wrap2, ErrConnectionRefused) {
		t.Error("Is should match by code in chain")
	}

	// Should not match unrelated error
	if Is(wrap2, errors.New("unrelated")) {
		t.Error("Is should not match unrelated error")
	}
}

func TestIsWithSentinelErrors(t *testing.T) {
	// Test direct sentinel match
	if !Is(ErrNotFound, ErrNotFound) {
		t.Error("Is should match identical sentinel errors")
	}

	// Test code-based match with different instances
	err1 := New(CodeNotFound, "resource not found")
	if !Is(err1, ErrNotFound) {
		t.Error("Is should match sentinel by code")
	}

	// Test wrapped sentinel match
	wrapped := Wrap(ErrTimeout, CodeInternal, "operation timed out")
	if !Is(wrapped, ErrTimeout) {
		t.Error("Is should match sentinel in wrapped error")
	}

	// Test multiple sentinel types
	if !Is(ErrBackendNotFound, ErrBackendNotFound) {
		t.Error("Is should match ErrBackendNotFound sentinel")
	}

	if !Is(ErrPoolNotFound, ErrPoolNotFound) {
		t.Error("Is should match ErrPoolNotFound sentinel")
	}

	// Different sentinels should not match
	if Is(ErrNotFound, ErrBackendNotFound) {
		t.Error("Is should not match different sentinels")
	}
}

func TestAsWithWrappedErrors(t *testing.T) {
	// Create a chain of wrapped errors
	root := New(CodeNotFound, "not found")
	wrap1 := Wrap(root, CodeInternal, "internal error")
	wrap2 := Wrap(wrap1, CodeUnavailable, "unavailable")

	// Should extract the outermost *Error from chain
	var target *Error
	if !As(wrap2, &target) {
		t.Error("As should succeed for wrapped error chain")
	}
	if target.Code != CodeUnavailable {
		t.Errorf("As extracted error with code %d, want %d", target.Code, CodeUnavailable)
	}

	// Should extract from standard error wrapped with fmt.Errorf
	stdWrapped := fmt.Errorf("standard wrap: %w", root)
	target = nil
	if !As(stdWrapped, &target) {
		t.Error("As should work with fmt.Errorf wrapping")
	}
	if target.Code != CodeNotFound {
		t.Errorf("As extracted error with code %d, want %d", target.Code, CodeNotFound)
	}
}

func TestUnwrapMultipleLayers(t *testing.T) {
	// Create a 3-layer chain using New for the root
	layer1 := New(CodeInternal, "layer 1")
	layer2 := Wrap(layer1, CodeNotFound, "layer 2")
	layer3 := Wrap(layer2, CodeUnavailable, "layer 3")

	// Unwrap layer3 should get layer2
	unwrapped1 := layer3.Unwrap()
	if unwrapped1 != layer2 {
		t.Error("First Unwrap should return layer2")
	}

	// Unwrap layer2 should get layer1
	unwrapped2 := layer2.Unwrap()
	if unwrapped2 != layer1 {
		t.Error("Second Unwrap should return layer1")
	}

	// Unwrap layer1 (New error) should get nil
	unwrapped3 := layer1.Unwrap()
	if unwrapped3 != nil {
		t.Errorf("Unwrap of New error should be nil, got %v", unwrapped3)
	}

	// Verify full chain traversal with Is
	if !Is(layer3, layer1) {
		t.Error("Is should traverse full unwrap chain")
	}

	// Also test with standard error in the chain
	stdErr := errors.New("standard error")
	wrappedStd := Wrap(stdErr, CodeInternal, "wrapped standard")
	doubleWrapped := Wrap(wrappedStd, CodeNotFound, "double wrapped")

	// Should be able to find standard error in chain
	if !Is(doubleWrapped, stdErr) {
		t.Error("Is should find standard error in chain")
	}

	// Unwrap should return the wrapped standard error
	if doubleWrapped.Unwrap() != wrappedStd {
		t.Error("Unwrap should return wrapped standard error")
	}
}

func TestErrorCodeMethods(t *testing.T) {
	// Test String() method for all defined codes
	tests := []struct {
		code     Code
		expected string
	}{
		{CodeUnknown, "UNKNOWN"},
		{CodeInternal, "INTERNAL"},
		{CodeInvalidArg, "INVALID_ARGUMENT"},
		{CodeNotFound, "NOT_FOUND"},
		{CodeAlreadyExist, "ALREADY_EXISTS"},
		{CodeUnavailable, "UNAVAILABLE"},
		{CodeTimeout, "TIMEOUT"},
		{CodeCanceled, "CANCELED"},
		{CodeConfigInvalid, "CONFIG_INVALID"},
		{CodeBackendNotFound, "BACKEND_NOT_FOUND"},
		{CodeConnectionRefused, "CONNECTION_REFUSED"},
		{CodeRateLimitExceeded, "RATE_LIMIT_EXCEEDED"},
		{CodeClusterNotReady, "CLUSTER_NOT_READY"},
		{Code(999), "CODE_999"},
		{Code(0), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.code.String()
		if got != tt.expected {
			t.Errorf("Code(%d).String() = %s, want %s", tt.code, got, tt.expected)
		}
	}
}

func TestErrorWithCodeFormatting(t *testing.T) {
	// Test Error() with no cause
	err := New(CodeNotFound, "resource not found")
	expected := "NOT_FOUND: resource not found"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// Test Error() with cause
	cause := errors.New("underlying problem")
	wrapped := Wrap(cause, CodeInternal, "operation failed")
	expected = "INTERNAL: operation failed: underlying problem"
	if wrapped.Error() != expected {
		t.Errorf("Error() = %s, want %s", wrapped.Error(), expected)
	}

	// Test Error() with wrapped *Error cause
	inner := New(CodeConnectionRefused, "connection refused")
	outer := Wrap(inner, CodeBackendUnavailable, "backend down")
	expected = "BACKEND_UNAVAILABLE: backend down: CONNECTION_REFUSED: connection refused"
	if outer.Error() != expected {
		t.Errorf("Error() = %s, want %s", outer.Error(), expected)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test CodeOf with various inputs
	t.Run("CodeOf", func(t *testing.T) {
		// Nil error returns CodeUnknown
		if CodeOf(nil) != CodeUnknown {
			t.Errorf("CodeOf(nil) = %d, want %d", CodeOf(nil), CodeUnknown)
		}

		// *Error returns its code
		err := New(CodeNotFound, "test")
		if CodeOf(err) != CodeNotFound {
			t.Errorf("CodeOf(err) = %d, want %d", CodeOf(err), CodeNotFound)
		}

		// Standard error returns CodeUnknown
		stdErr := errors.New("standard")
		if CodeOf(stdErr) != CodeUnknown {
			t.Errorf("CodeOf(stdErr) = %d, want %d", CodeOf(stdErr), CodeUnknown)
		}
	})

	// Test IsCode
	t.Run("IsCode", func(t *testing.T) {
		err := New(CodeTimeout, "timeout")
		if !IsCode(err, CodeTimeout) {
			t.Error("IsCode should match correct code")
		}
		if IsCode(err, CodeNotFound) {
			t.Error("IsCode should not match different code")
		}
		if IsCode(nil, CodeTimeout) {
			t.Error("IsCode(nil) should be false")
		}
	})

	// Test WithContext helper
	t.Run("WithContext", func(t *testing.T) {
		// With *Error
		err := New(CodeInternal, "error")
		result := WithContext(err, "request_id", "abc123")
		if e, ok := result.(*Error); !ok {
			t.Error("WithContext should return *Error")
		} else if e.Context["request_id"] != "abc123" {
			t.Errorf("Context['request_id'] = %v, want 'abc123'", e.Context["request_id"])
		}

		// With nil
		if WithContext(nil, "key", "value") != nil {
			t.Error("WithContext(nil) should return nil")
		}

		// With standard error (returns as-is)
		stdErr := errors.New("standard")
		if WithContext(stdErr, "key", "value") != stdErr {
			t.Error("WithContext should return as-is for non-*Error")
		}
	})
}

func TestErrorIsMethod(t *testing.T) {
	// Test the Is method on *Error type
	err1 := New(CodeNotFound, "not found")
	err2 := New(CodeNotFound, "also not found")
	err3 := New(CodeInternal, "internal")

	// Same code should match
	if !err1.Is(err2) {
		t.Error("Is should match errors with same code")
	}

	// Different code should not match
	if err1.Is(err3) {
		t.Error("Is should not match errors with different codes")
	}

	// Nil target should return false
	if err1.Is(nil) {
		t.Error("Is(nil) should be false")
	}

	// Wrapped error should match by code
	wrapped := Wrap(err1, CodeInternal, "wrapped")
	if !wrapped.Is(err1) {
		t.Error("Is should match wrapped error by code")
	}

	// Should match sentinel
	if !err1.Is(ErrNotFound) {
		t.Error("Is should match sentinel with same code")
	}
}

func TestAsPanic(t *testing.T) {
	// As should panic with nil target
	defer func() {
		if r := recover(); r == nil {
			t.Error("As should panic with nil target")
		}
	}()

	_ = As(New(CodeNotFound, "test"), nil)
}

// customErrorForTest is a custom error type for testing
type customErrorForTest struct {
	msg string
}

func (c *customErrorForTest) Error() string { return c.msg }

// TestUnwrapWithCustomErrorTypes tests unwrapping with custom error types
func TestUnwrapWithCustomErrorTypes(t *testing.T) {
	customErr := &customErrorForTest{msg: "custom error"}

	// Wrap the custom error
	wrapped := Wrap(customErr, CodeInternal, "wrapped custom error")

	// Unwrap should return the custom error
	unwrapped := wrapped.Unwrap()
	if unwrapped != customErr {
		t.Error("Unwrap should return the custom error")
	}

	// Is should find the custom error in the chain
	if !Is(wrapped, customErr) {
		t.Error("Is should find custom error in chain")
	}

	// Test with standard error wrapped in *Error
	stdErr := New(CodeNotFound, "standard olb error")
	doubleWrapped := Wrap(stdErr, CodeInternal, "double wrapped")

	// Should find the inner *Error by code
	if !Is(doubleWrapped, ErrNotFound) {
		t.Error("Is should find inner *Error by code")
	}

	// Unwrap chain
	if doubleWrapped.Unwrap() != stdErr {
		t.Error("First unwrap should return stdErr")
	}
}

// TestIsWithMultipleWrappedErrors tests Is with deeply nested error chains
func TestIsWithMultipleWrappedErrors(t *testing.T) {
	// Create a chain: root -> wrap1 -> wrap2 -> wrap3
	root := errors.New("root cause")
	wrap1 := Wrap(root, CodeConnectionRefused, "layer 1")
	wrap2 := Wrap(wrap1, CodeBackendUnavailable, "layer 2")
	wrap3 := Wrap(wrap2, CodeInternal, "layer 3")

	// Should find all levels
	if !Is(wrap3, root) {
		t.Error("Should find root")
	}
	if !Is(wrap3, wrap1) {
		t.Error("Should find wrap1")
	}
	if !Is(wrap3, wrap2) {
		t.Error("Should find wrap2")
	}
	if !Is(wrap3, wrap3) {
		t.Error("Should find wrap3 (itself)")
	}

	// Should find by code at any level
	if !Is(wrap3, ErrConnectionRefused) {
		t.Error("Should find ErrConnectionRefused code")
	}
	if !Is(wrap3, ErrBackendUnavailable) {
		t.Error("Should find ErrBackendUnavailable code")
	}
	if !Is(wrap3, ErrInternal) {
		t.Error("Should find ErrInternal code")
	}

	// Should not match unrelated codes
	if Is(wrap3, ErrNotFound) {
		t.Error("Should not match ErrNotFound")
	}
	if Is(wrap3, ErrTimeout) {
		t.Error("Should not match ErrTimeout")
	}

	// Test with mixed standard and olb errors
	stdWrap := fmt.Errorf("standard: %w", wrap3)
	if !Is(stdWrap, root) {
		t.Error("Should find root through standard error wrap")
	}
	if !Is(stdWrap, ErrConnectionRefused) {
		t.Error("Should find code through standard error wrap")
	}
}

// TestErrorfWithComplexFormat tests Errorf (Newf) with complex formatting
func TestErrorfWithComplexFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		expected string
		code     Code
	}{
		{
			name:     "multiple strings",
			format:   "backend %s on host %s failed",
			args:     []any{"web01", "server1"},
			expected: "INTERNAL: backend web01 on host server1 failed",
			code:     CodeInternal,
		},
		{
			name:     "mixed types",
			format:   "request %s took %d ms, status %d",
			args:     []any{"/api/users", 150, 200},
			expected: "TIMEOUT: request /api/users took 150 ms, status 200",
			code:     CodeTimeout,
		},
		{
			name:     "float formatting",
			format:   "latency: %.2f ms, rate: %.4f",
			args:     []any{123.456, 0.1234},
			expected: "RATE_LIMIT_EXCEEDED: latency: 123.46 ms, rate: 0.1234",
			code:     CodeRateLimitExceeded,
		},
		{
			name:     "boolean formatting",
			format:   "healthy=%v, enabled=%t",
			args:     []any{true, false},
			expected: "BACKEND_UNHEALTHY: healthy=true, enabled=false",
			code:     CodeBackendUnhealthy,
		},
		{
			name:     "hex formatting",
			format:   "address: 0x%x, decimal: %d",
			args:     []any{255, 255},
			expected: "INTERNAL: address: 0xff, decimal: 255",
			code:     CodeInternal,
		},
		{
			name:     "quoted string",
			format:   "invalid input: %q",
			args:     []any{"hello world"},
			expected: "INVALID_ARGUMENT: invalid input: \"hello world\"",
			code:     CodeInvalidArg,
		},
		{
			name:     "width and padding",
			format:   "id: %05d, name: %10s",
			args:     []any{42, "test"},
			expected: "NOT_FOUND: id: 00042, name:       test",
			code:     CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.code, tt.format, tt.args...)
			if err.Error() != tt.expected {
				t.Errorf("Error() = %s, want %s", err.Error(), tt.expected)
			}
			if err.Code != tt.code {
				t.Errorf("Code = %d, want %d", err.Code, tt.code)
			}
		})
	}
}

// TestCodeFromError tests CodeFromError (CodeOf) for all error types
func TestCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected Code
	}{
		// Sentinel errors
		{"ErrUnknown", ErrUnknown, CodeUnknown},
		{"ErrInternal", ErrInternal, CodeInternal},
		{"ErrInvalidArg", ErrInvalidArg, CodeInvalidArg},
		{"ErrNotFound", ErrNotFound, CodeNotFound},
		{"ErrAlreadyExist", ErrAlreadyExist, CodeAlreadyExist},
		{"ErrUnavailable", ErrUnavailable, CodeUnavailable},
		{"ErrTimeout", ErrTimeout, CodeTimeout},
		{"ErrCanceled", ErrCanceled, CodeCanceled},
		{"ErrConfigInvalid", ErrConfigInvalid, CodeConfigInvalid},
		{"ErrConfigParseError", ErrConfigParseError, CodeConfigParseError},
		{"ErrConfigNotFound", ErrConfigNotFound, CodeConfigNotFound},
		{"ErrConfigValidation", ErrConfigValidation, CodeConfigValidation},
		{"ErrBackendNotFound", ErrBackendNotFound, CodeBackendNotFound},
		{"ErrPoolNotFound", ErrPoolNotFound, CodePoolNotFound},
		{"ErrRouteNotFound", ErrRouteNotFound, CodeRouteNotFound},
		{"ErrBackendUnavailable", ErrBackendUnavailable, CodeBackendUnavailable},
		{"ErrBackendUnhealthy", ErrBackendUnhealthy, CodeBackendUnhealthy},
		{"ErrPoolEmpty", ErrPoolEmpty, CodePoolEmpty},
		{"ErrConnectionRefused", ErrConnectionRefused, CodeConnectionRefused},
		{"ErrConnectionReset", ErrConnectionReset, CodeConnectionReset},
		{"ErrConnectionTimeout", ErrConnectionTimeout, CodeConnectionTimeout},
		{"ErrConnectionClosed", ErrConnectionClosed, CodeConnectionClosed},
		{"ErrTLSHandshake", ErrTLSHandshake, CodeTLSHandshake},
		{"ErrCertificate", ErrCertificate, CodeCertificate},
		{"ErrProtocolError", ErrProtocolError, CodeProtocolError},
		{"ErrInvalidRequest", ErrInvalidRequest, CodeInvalidRequest},
		{"ErrInvalidResponse", ErrInvalidResponse, CodeInvalidResponse},
		{"ErrHTTPError", ErrHTTPError, CodeHTTPError},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, CodeRateLimitExceeded},
		{"ErrCircuitOpen", ErrCircuitOpen, CodeCircuitOpen},
		{"ErrAuthRequired", ErrAuthRequired, CodeAuthRequired},
		{"ErrAuthFailed", ErrAuthFailed, CodeAuthFailed},
		{"ErrForbidden", ErrForbidden, CodeForbidden},
		{"ErrWAFBlocked", ErrWAFBlocked, CodeWAFBlocked},
		{"ErrClusterNotReady", ErrClusterNotReady, CodeClusterNotReady},
		{"ErrNodeNotFound", ErrNodeNotFound, CodeNodeNotFound},
		{"ErrRaftError", ErrRaftError, CodeRaftError},
		{"ErrGossipError", ErrGossipError, CodeGossipError},
		// Other error types
		{"nil error", nil, CodeUnknown},
		{"standard error", errors.New("standard"), CodeUnknown},
		{"custom *Error", New(Code(999), "custom"), Code(999)},
		{"wrapped error", Wrap(ErrNotFound, CodeInternal, "wrapped"), CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CodeOf(tt.err)
			if got != tt.expected {
				t.Errorf("CodeOf() = %d (%s), want %d (%s)",
					got, got.String(), tt.expected, tt.expected.String())
			}
		})
	}
}

// TestErrorTypesAndMessages tests all sentinel error types and their messages
func TestErrorTypesAndMessages(t *testing.T) {
	tests := []struct {
		err      *Error
		expected string
	}{
		{ErrUnknown, "UNKNOWN: unknown error"},
		{ErrInternal, "INTERNAL: internal error"},
		{ErrInvalidArg, "INVALID_ARGUMENT: invalid argument"},
		{ErrNotFound, "NOT_FOUND: not found"},
		{ErrAlreadyExist, "ALREADY_EXISTS: already exists"},
		{ErrUnavailable, "UNAVAILABLE: unavailable"},
		{ErrTimeout, "TIMEOUT: timeout"},
		{ErrCanceled, "CANCELED: canceled"},
		{ErrConfigInvalid, "CONFIG_INVALID: invalid configuration"},
		{ErrConfigParseError, "CONFIG_PARSE_ERROR: configuration parse error"},
		{ErrConfigNotFound, "CONFIG_NOT_FOUND: configuration not found"},
		{ErrConfigValidation, "CONFIG_VALIDATION: configuration validation failed"},
		{ErrBackendNotFound, "BACKEND_NOT_FOUND: backend not found"},
		{ErrPoolNotFound, "POOL_NOT_FOUND: pool not found"},
		{ErrRouteNotFound, "ROUTE_NOT_FOUND: route not found"},
		{ErrBackendUnavailable, "BACKEND_UNAVAILABLE: backend unavailable"},
		{ErrBackendUnhealthy, "BACKEND_UNHEALTHY: backend unhealthy"},
		{ErrPoolEmpty, "POOL_EMPTY: pool is empty"},
		{ErrConnectionRefused, "CONNECTION_REFUSED: connection refused"},
		{ErrConnectionReset, "CONNECTION_RESET: connection reset"},
		{ErrConnectionTimeout, "CONNECTION_TIMEOUT: connection timeout"},
		{ErrConnectionClosed, "CONNECTION_CLOSED: connection closed"},
		{ErrTLSHandshake, "TLS_HANDSHAKE: TLS handshake failed"},
		{ErrCertificate, "CERTIFICATE: certificate error"},
		{ErrProtocolError, "PROTOCOL_ERROR: protocol error"},
		{ErrInvalidRequest, "INVALID_REQUEST: invalid request"},
		{ErrInvalidResponse, "INVALID_RESPONSE: invalid response"},
		{ErrHTTPError, "HTTP_ERROR: HTTP error"},
		{ErrRateLimitExceeded, "RATE_LIMIT_EXCEEDED: rate limit exceeded"},
		{ErrCircuitOpen, "CIRCUIT_OPEN: circuit breaker open"},
		{ErrAuthRequired, "AUTH_REQUIRED: authentication required"},
		{ErrAuthFailed, "AUTH_FAILED: authentication failed"},
		{ErrForbidden, "FORBIDDEN: forbidden"},
		{ErrWAFBlocked, "WAF_BLOCKED: blocked by WAF"},
		{ErrClusterNotReady, "CLUSTER_NOT_READY: cluster not ready"},
		{ErrNodeNotFound, "NODE_NOT_FOUND: node not found"},
		{ErrRaftError, "RAFT_ERROR: raft error"},
		{ErrGossipError, "GOSSIP_ERROR: gossip error"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Code.String(), func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Error() = %s, want %s", tt.err.Error(), tt.expected)
			}
		})
	}
}

// TestErrorCauseChaining tests error cause chaining behavior
func TestErrorCauseChaining(t *testing.T) {
	// Test simple chain
	t.Run("simple chain", func(t *testing.T) {
		root := errors.New("root")
		layer1 := Wrap(root, CodeInternal, "layer1")
		layer2 := Wrap(layer1, CodeNotFound, "layer2")

		// Check chain traversal
		if !Is(layer2, root) {
			t.Error("Should find root through chain")
		}
		if !Is(layer2, layer1) {
			t.Error("Should find layer1 through chain")
		}

		// Check error message includes cause
		msg := layer2.Error()
		if msg == "" {
			t.Error("Error message should not be empty")
		}
	})

	// Test chain with *Error causes
	t.Run("olb error chain", func(t *testing.T) {
		root := New(CodeConnectionRefused, "connection refused")
		layer1 := Wrap(root, CodeBackendUnavailable, "backend unavailable")
		layer2 := Wrap(layer1, CodeInternal, "internal error")

		// Should find by code at any level
		if !Is(layer2, ErrConnectionRefused) {
			t.Error("Should find ErrConnectionRefused")
		}
		if !Is(layer2, ErrBackendUnavailable) {
			t.Error("Should find ErrBackendUnavailable")
		}
		if !Is(layer2, ErrInternal) {
			t.Error("Should find ErrInternal")
		}

		// Error message should show full chain
		expected := "INTERNAL: internal error: BACKEND_UNAVAILABLE: backend unavailable: CONNECTION_REFUSED: connection refused"
		if layer2.Error() != expected {
			t.Errorf("Error() = %s, want %s", layer2.Error(), expected)
		}
	})

	// Test chain with mixed error types
	t.Run("mixed chain", func(t *testing.T) {
		std1 := errors.New("standard error 1")
		olb1 := Wrap(std1, CodeInternal, "olb layer 1")
		std2 := fmt.Errorf("wrap: %w", olb1)
		olb2 := Wrap(std2, CodeNotFound, "olb layer 2")

		// Should traverse entire chain
		if !Is(olb2, std1) {
			t.Error("Should find std1 through mixed chain")
		}
		if !Is(olb2, olb1) {
			t.Error("Should find olb1 through mixed chain")
		}
		if !Is(olb2, ErrInternal) {
			t.Error("Should find ErrInternal code")
		}
		if !Is(olb2, ErrNotFound) {
			t.Error("Should find ErrNotFound code")
		}
	})

	// Test Unwrap returns correct cause at each level
	t.Run("unwrap chain", func(t *testing.T) {
		root := New(CodeConnectionRefused, "root")
		layer1 := Wrap(root, CodeInternal, "layer1")
		layer2 := Wrap(layer1, CodeNotFound, "layer2")

		if layer2.Unwrap() != layer1 {
			t.Error("layer2.Unwrap() should be layer1")
		}
		if layer1.Unwrap() != root {
			t.Error("layer1.Unwrap() should be root")
		}
		if root.Unwrap() != nil {
			t.Error("root.Unwrap() should be nil")
		}
	})
}

// TestAsWithInterfaceTargets tests As with various interface targets
func TestAsWithInterfaceTargets(t *testing.T) {
	// Test As with *Error target
	t.Run("as *Error", func(t *testing.T) {
		err := New(CodeNotFound, "test")
		var target *Error
		if !As(err, &target) {
			t.Error("As should succeed for *Error")
		}
		if target != err {
			t.Error("target should be the original error")
		}
	})

	// Test As with wrapped error
	t.Run("as wrapped *Error", func(t *testing.T) {
		inner := New(CodeConnectionRefused, "inner")
		outer := Wrap(inner, CodeInternal, "outer")

		var target *Error
		if !As(outer, &target) {
			t.Error("As should succeed for wrapped *Error")
		}
		if target != outer {
			t.Error("As should extract the outermost *Error")
		}
		if target.Code != CodeInternal {
			t.Errorf("target.Code = %d, want %d", target.Code, CodeInternal)
		}
	})

	// Test As with standard error wrapping
	t.Run("as through fmt.Errorf", func(t *testing.T) {
		inner := New(CodeNotFound, "inner")
		wrapped := fmt.Errorf("wrapped: %w", inner)

		var target *Error
		if !As(wrapped, &target) {
			t.Error("As should succeed through fmt.Errorf wrapping")
		}
		if target != inner {
			t.Error("target should be the inner error")
		}
	})

	// Test As with error interface (should fail - not a pointer to *Error)
	t.Run("as error interface", func(t *testing.T) {
		err := New(CodeNotFound, "test")
		var target error
		if As(err, &target) {
			t.Error("As should fail for error interface target")
		}
	})

	// Test As with non-matching type
	t.Run("as non-matching type", func(t *testing.T) {
		err := errors.New("standard error")
		var target *Error
		if As(err, &target) {
			t.Error("As should fail for standard error")
		}
	})

	// Test As with deeply nested chain
	t.Run("as deeply nested", func(t *testing.T) {
		level1 := New(CodeConnectionRefused, "level1")
		level2 := Wrap(level1, CodeBackendUnavailable, "level2")
		level3 := Wrap(level2, CodeInternal, "level3")

		var target *Error
		if !As(level3, &target) {
			t.Error("As should succeed for deeply nested chain")
		}
		// Should get the outermost *Error
		if target.Code != CodeInternal {
			t.Errorf("target.Code = %d, want %d", target.Code, CodeInternal)
		}
	})
}

// TestErrorContextWithChaining tests context with error chaining
func TestErrorContextWithChaining(t *testing.T) {
	// Test context on wrapped error
	err := New(CodeNotFound, "not found").
		WithContext("key1", "value1").
		WithContext("key2", 42)

	if err.Context["key1"] != "value1" {
		t.Error("Context key1 should be value1")
	}
	if err.Context["key2"] != 42 {
		t.Error("Context key2 should be 42")
	}

	// Wrap the error with context
	wrapped := Wrap(err, CodeInternal, "wrapped")
	wrappedWithContext := wrapped.WithContext("key3", true)

	if wrappedWithContext.Context["key3"] != true {
		t.Error("Wrapped error should have its own context")
	}

	// Original error should not be modified
	if _, ok := err.Context["key3"]; ok {
		t.Error("Original error should not have key3")
	}
}

// TestAllErrorCodes tests that all defined codes have string representations
func TestAllErrorCodes(t *testing.T) {
	// Test all defined error codes have valid String() output
	codes := []Code{
		CodeUnknown,
		CodeInternal,
		CodeInvalidArg,
		CodeNotFound,
		CodeAlreadyExist,
		CodeUnavailable,
		CodeTimeout,
		CodeCanceled,
		CodeConfigInvalid,
		CodeConfigParseError,
		CodeConfigNotFound,
		CodeConfigValidation,
		CodeBackendNotFound,
		CodePoolNotFound,
		CodeRouteNotFound,
		CodeBackendUnavailable,
		CodeBackendUnhealthy,
		CodePoolEmpty,
		CodeConnectionRefused,
		CodeConnectionReset,
		CodeConnectionTimeout,
		CodeConnectionClosed,
		CodeTLSHandshake,
		CodeCertificate,
		CodeProtocolError,
		CodeInvalidRequest,
		CodeInvalidResponse,
		CodeHTTPError,
		CodeRateLimitExceeded,
		CodeCircuitOpen,
		CodeAuthRequired,
		CodeAuthFailed,
		CodeForbidden,
		CodeWAFBlocked,
		CodeClusterNotReady,
		CodeNodeNotFound,
		CodeRaftError,
		CodeGossipError,
	}

	for _, code := range codes {
		s := code.String()
		if s == "" {
			t.Errorf("Code %d has empty String()", code)
		}
		if s == fmt.Sprintf("CODE_%d", code) && code <= CodeGossipError {
			t.Errorf("Code %d should have a named String(), got %s", code, s)
		}
	}
}

// TestNilErrorHandling tests handling of nil errors
func TestNilErrorHandling(t *testing.T) {
	// Wrap nil should return nil
	if Wrap(nil, CodeInternal, "test") != nil {
		t.Error("Wrap(nil) should return nil")
	}
	if Wrapf(nil, CodeInternal, "test %s", "arg") != nil {
		t.Error("Wrapf(nil) should return nil")
	}

	// Is with nil
	if Is(nil, nil) {
		t.Error("Is(nil, nil) should be false")
	}
	if Is(New(CodeNotFound, "test"), nil) {
		t.Error("Is(err, nil) should be false")
	}

	// CodeOf nil
	if CodeOf(nil) != CodeUnknown {
		t.Error("CodeOf(nil) should be CodeUnknown")
	}

	// IsCode with nil - CodeOf(nil) returns CodeUnknown, so IsCode(nil, CodeUnknown) is true
	if !IsCode(nil, CodeUnknown) {
		t.Error("IsCode(nil, CodeUnknown) should be true")
	}
	if IsCode(nil, CodeInternal) {
		t.Error("IsCode(nil, CodeInternal) should be false")
	}

	// WithContext with nil
	if WithContext(nil, "key", "value") != nil {
		t.Error("WithContext(nil) should return nil")
	}

	// As with nil error (not target)
	var target *Error
	if As(nil, &target) {
		t.Error("As(nil, &target) should be false")
	}
}

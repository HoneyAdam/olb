package admin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdminCircuitBreaker_InitialState(t *testing.T) {
	cb := newAdminCircuitBreaker()
	if cb.State() != "closed" {
		t.Errorf("initial state = %q, want %q", cb.State(), "closed")
	}
}

func TestAdminCircuitBreaker_Execute_Success(t *testing.T) {
	cb := newAdminCircuitBreaker()
	err := cb.Execute(func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if cb.State() != "closed" {
		t.Errorf("state after success = %q, want %q", cb.State(), "closed")
	}
}

func TestAdminCircuitBreaker_Execute_Error(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 3

	for i := 0; i < 3; i++ {
		err := cb.Execute(func(ctx context.Context) error {
			return errors.New("fail")
		})
		if err == nil {
			t.Error("expected error from Execute")
		}
	}

	if cb.State() != "open" {
		t.Errorf("state after %d errors = %q, want %q", 3, cb.State(), "open")
	}
}

func TestAdminCircuitBreaker_OpenRejects(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 1

	// Trigger open state
	_ = cb.Execute(func(ctx context.Context) error {
		return errors.New("fail")
	})

	if cb.State() != "open" {
		t.Fatalf("state = %q, want open", cb.State())
	}

	// Next call should be rejected immediately
	err := cb.Execute(func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Error("expected error when circuit is open")
	}
}

func TestAdminCircuitBreaker_HalfOpen_Recovery(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 1
	cb.openDuration = 50 * time.Millisecond

	// Open the circuit
	_ = cb.Execute(func(ctx context.Context) error {
		return errors.New("fail")
	})

	// Wait for open duration to pass
	time.Sleep(60 * time.Millisecond)

	if cb.State() != "half-open" {
		t.Fatalf("state = %q, want half-open", cb.State())
	}

	// Successful calls should close it
	for i := 0; i < 3; i++ {
		err := cb.Execute(func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("half-open Execute() error = %v", err)
		}
	}

	if cb.State() != "closed" {
		t.Errorf("state after recovery = %q, want closed", cb.State())
	}
}

func TestAdminCircuitBreaker_HalfOpen_ReOpen(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 1
	cb.openDuration = 50 * time.Millisecond

	// Open the circuit
	_ = cb.Execute(func(ctx context.Context) error {
		return errors.New("fail")
	})

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open -> should re-open
	err := cb.Execute(func(ctx context.Context) error {
		return errors.New("still failing")
	})
	if err == nil {
		t.Error("expected error")
	}

	if cb.State() != "open" {
		t.Errorf("state after half-open failure = %q, want open", cb.State())
	}
}

func TestAdminCircuitBreaker_Timeout(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.timeout = 50 * time.Millisecond

	err := cb.Execute(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Error("expected timeout error")
	}
	if err.Error() != "admin call timed out after 50ms" {
		t.Errorf("error = %q, want timeout message", err.Error())
	}
}

func TestAdminCircuitBreaker_Reset(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 1

	// Open it
	_ = cb.Execute(func(ctx context.Context) error {
		return errors.New("fail")
	})
	if cb.State() != "open" {
		t.Fatalf("state = %q, want open", cb.State())
	}

	cb.Reset()

	if cb.State() != "closed" {
		t.Errorf("state after reset = %q, want closed", cb.State())
	}
}

func TestAdminCircuitBreaker_ConcurrentSafe(t *testing.T) {
	cb := newAdminCircuitBreaker()
	cb.errorThreshold = 100
	cb.timeout = 5 * time.Second

	var successes, failures atomic.Int64
	done := make(chan struct{})

	for i := 0; i < 50; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			err := cb.Execute(func(ctx context.Context) error {
				if idx%3 == 0 {
					return errors.New("fail")
				}
				return nil
			})
			if err == nil {
				successes.Add(1)
			} else {
				failures.Add(1)
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	total := successes.Load() + failures.Load()
	if total != 50 {
		t.Errorf("total outcomes = %d, want 50", total)
	}
}

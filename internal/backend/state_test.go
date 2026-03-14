package backend

import (
	"testing"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateStarting, "starting"},
		{StateUp, "up"},
		{StateDown, "down"},
		{StateDraining, "draining"},
		{StateMaintenance, "maintenance"},
		{State(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStateIsAvailable(t *testing.T) {
	tests := []struct {
		state     State
		available bool
	}{
		{StateStarting, false},
		{StateUp, true},
		{StateDown, false},
		{StateDraining, false},
		{StateMaintenance, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsAvailable(); got != tt.available {
				t.Errorf("State.IsAvailable() = %v, want %v", got, tt.available)
			}
		})
	}
}

func TestStateIsActive(t *testing.T) {
	tests := []struct {
		state  State
		active bool
	}{
		{StateStarting, false},
		{StateUp, true},
		{StateDown, false},
		{StateDraining, true},
		{StateMaintenance, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsActive(); got != tt.active {
				t.Errorf("State.IsActive() = %v, want %v", got, tt.active)
			}
		})
	}
}

func TestStateCanTransitionTo(t *testing.T) {
	tests := []struct {
		from     State
		to       State
		expected bool
	}{
		// Starting transitions
		{StateStarting, StateUp, true},
		{StateStarting, StateDown, true},
		{StateStarting, StateMaintenance, true},
		{StateStarting, StateDraining, false},

		// Up transitions
		{StateUp, StateDown, true},
		{StateUp, StateDraining, true},
		{StateUp, StateMaintenance, true},
		{StateUp, StateStarting, false},

		// Down transitions
		{StateDown, StateUp, true},
		{StateDown, StateMaintenance, true},
		{StateDown, StateDraining, false},

		// Draining transitions
		{StateDraining, StateDown, true},
		{StateDraining, StateMaintenance, true},
		{StateDraining, StateUp, false},

		// Maintenance transitions
		{StateMaintenance, StateUp, true},
		{StateMaintenance, StateDown, true},
		{StateMaintenance, StateDraining, false},
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"_to_"+tt.to.String(), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.expected {
				t.Errorf("State.CanTransitionTo() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAtomicState(t *testing.T) {
	as := NewAtomicState(StateStarting)

	if got := as.Load(); got != StateStarting {
		t.Errorf("AtomicState.Load() = %v, want %v", got, StateStarting)
	}

	as.Store(StateUp)
	if got := as.Load(); got != StateUp {
		t.Errorf("AtomicState.Load() after Store = %v, want %v", got, StateUp)
	}

	// Test CompareAndSwap
	if !as.CompareAndSwap(StateUp, StateDraining) {
		t.Error("AtomicState.CompareAndSwap() should succeed")
	}
	if as.Load() != StateDraining {
		t.Errorf("AtomicState.Load() after CAS = %v, want %v", as.Load(), StateDraining)
	}

	// Test failed CompareAndSwap
	if as.CompareAndSwap(StateUp, StateMaintenance) {
		t.Error("AtomicState.CompareAndSwap() should fail with wrong old value")
	}
}

func TestAtomicStateTransition(t *testing.T) {
	as := NewAtomicState(StateStarting)

	// Valid transition
	if !as.Transition(StateUp) {
		t.Error("Transition(Starting -> Up) should succeed")
	}
	if as.Load() != StateUp {
		t.Errorf("State after transition = %v, want %v", as.Load(), StateUp)
	}

	// Invalid transition
	if as.Transition(StateStarting) {
		t.Error("Transition(Up -> Starting) should fail")
	}
	if as.Load() != StateUp {
		t.Errorf("State after failed transition = %v, want %v", as.Load(), StateUp)
	}

	// Another valid transition
	if !as.Transition(StateDraining) {
		t.Error("Transition(Up -> Draining) should succeed")
	}
}

func TestAtomicStateConcurrent(t *testing.T) {
	as := NewAtomicState(StateStarting)

	done := make(chan bool, 2)

	// Goroutine 1: try to transition to Up
	go func() {
		as.Transition(StateUp)
		done <- true
	}()

	// Goroutine 2: try to transition to Down
	go func() {
		as.Transition(StateDown)
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// State should be one of the valid transitions from Starting
	state := as.Load()
	if state != StateUp && state != StateDown {
		t.Errorf("Final state = %v, want Up or Down", state)
	}
}

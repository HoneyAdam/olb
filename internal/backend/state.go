package backend

import (
	"fmt"
	"sync/atomic"
)

// State represents the current state of a backend.
type State int32

const (
	// StateStarting indicates the backend is initializing.
	StateStarting State = iota
	// StateUp indicates the backend is healthy and accepting traffic.
	StateUp
	// StateDown indicates the backend is unhealthy and not accepting traffic.
	StateDown
	// StateDraining indicates the backend is gracefully draining connections.
	StateDraining
	// StateMaintenance indicates the backend is in maintenance mode.
	StateMaintenance
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateUp:
		return "up"
	case StateDown:
		return "down"
	case StateDraining:
		return "draining"
	case StateMaintenance:
		return "maintenance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// IsAvailable returns true if the backend can accept new connections.
func (s State) IsAvailable() bool {
	return s == StateUp
}

// IsActive returns true if the backend has active connections.
// This includes Up, Draining, and Maintenance states.
func (s State) IsActive() bool {
	return s == StateUp || s == StateDraining || s == StateMaintenance
}

// CanTransitionTo returns true if a state transition is valid.
func (s State) CanTransitionTo(newState State) bool {
	switch s {
	case StateStarting:
		// Starting can transition to Up, Down, or Maintenance
		return newState == StateUp || newState == StateDown || newState == StateMaintenance
	case StateUp:
		// Up can transition to Down, Draining, or Maintenance
		return newState == StateDown || newState == StateDraining || newState == StateMaintenance
	case StateDown:
		// Down can transition to Up or Maintenance
		return newState == StateUp || newState == StateMaintenance
	case StateDraining:
		// Draining can transition to Down or Maintenance
		return newState == StateDown || newState == StateMaintenance
	case StateMaintenance:
		// Maintenance can transition to Up or Down
		return newState == StateUp || newState == StateDown
	default:
		return false
	}
}

// AtomicState provides thread-safe state operations.
type AtomicState struct {
	v atomic.Int32
}

// NewAtomicState creates a new AtomicState with the given initial state.
func NewAtomicState(initial State) *AtomicState {
	as := &AtomicState{}
	as.Store(initial)
	return as
}

// Load atomically loads and returns the state.
func (as *AtomicState) Load() State {
	return State(as.v.Load())
}

// Store atomically stores the state.
func (as *AtomicState) Store(s State) {
	as.v.Store(int32(s))
}

// CompareAndSwap executes compare-and-swap on the state.
func (as *AtomicState) CompareAndSwap(old, new State) bool {
	return as.v.CompareAndSwap(int32(old), int32(new))
}

// Transition attempts to transition to a new state.
// Returns true if the transition was successful.
func (as *AtomicState) Transition(newState State) bool {
	for {
		current := as.Load()
		if !current.CanTransitionTo(newState) {
			return false
		}
		if as.CompareAndSwap(current, newState) {
			return true
		}
	}
}

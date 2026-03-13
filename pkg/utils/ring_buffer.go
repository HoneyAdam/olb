package utils

import (
	"sync/atomic"
)

// RingBuffer is a lock-free circular buffer for single-producer single-consumer (SPSC) scenarios.
// It uses atomic operations for synchronization, making it suitable for high-throughput
// inter-goroutine communication without locks.
//
// Type parameters:
//   - T: the element type stored in the buffer
//
// The capacity must be a power of two for efficient modulo operations.
// The buffer is thread-safe for one writer and one reader concurrently.
type RingBuffer[T any] struct {
	_ [0]func() // prevent equality comparison

	buffer []T
	mask   uint64 // capacity - 1, for fast modulo

	// head is the next position to read from (consumer)
	// tail is the next position to write to (producer)
	head atomic.Uint64
	tail atomic.Uint64
}

// NewRingBuffer creates a new ring buffer with the given capacity.
// The actual capacity will be rounded up to the next power of two.
// Returns an error if capacity is 0.
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	// Round up to power of two
	cap64 := uint64(capacity)
	cap64--
	cap64 |= cap64 >> 1
	cap64 |= cap64 >> 2
	cap64 |= cap64 >> 4
	cap64 |= cap64 >> 8
	cap64 |= cap64 >> 16
	cap64 |= cap64 >> 32
	cap64++

	return &RingBuffer[T]{
		buffer: make([]T, cap64),
		mask:   cap64 - 1,
	}, nil
}

// MustNewRingBuffer creates a new ring buffer, panicking on error.
func MustNewRingBuffer[T any](capacity int) *RingBuffer[T] {
	rb, err := NewRingBuffer[T](capacity)
	if err != nil {
		panic(err)
	}
	return rb
}

// Capacity returns the actual capacity of the buffer (always a power of two).
func (rb *RingBuffer[T]) Capacity() int {
	return len(rb.buffer)
}

// Len returns the number of elements currently in the buffer.
// Note: This is an approximation as head and tail may change during the call.
// For SPSC scenarios, this is safe to call from either goroutine.
func (rb *RingBuffer[T]) Len() int {
	tail := rb.tail.Load()
	head := rb.head.Load()
	return int(tail - head)
}

// IsEmpty returns true if the buffer contains no elements.
func (rb *RingBuffer[T]) IsEmpty() bool {
	return rb.head.Load() == rb.tail.Load()
}

// IsFull returns true if the buffer is at capacity.
func (rb *RingBuffer[T]) IsFull() bool {
	return rb.Len() == len(rb.buffer)
}

// Push adds an element to the buffer.
// Returns true if successful, false if the buffer is full.
// Must only be called from the producer goroutine.
func (rb *RingBuffer[T]) Push(val T) bool {
	tail := rb.tail.Load()
	head := rb.head.Load()

	// Check if full
	if tail-head >= uint64(len(rb.buffer)) {
		return false
	}

	// Write to buffer
	rb.buffer[tail&rb.mask] = val

	// Update tail (publish the write)
	rb.tail.Store(tail + 1)

	return true
}

// Pop removes and returns an element from the buffer.
// Returns the value and true if successful, zero value and false if empty.
// Must only be called from the consumer goroutine.
func (rb *RingBuffer[T]) Pop() (T, bool) {
	head := rb.head.Load()
	tail := rb.tail.Load()

	// Check if empty
	if head == tail {
		var zero T
		return zero, false
	}

	// Read from buffer
	val := rb.buffer[head&rb.mask]

	// Update head (publish the read)
	rb.head.Store(head + 1)

	return val, true
}

// TryPush is an alias for Push.
func (rb *RingBuffer[T]) TryPush(val T) bool {
	return rb.Push(val)
}

// TryPop is an alias for Pop.
func (rb *RingBuffer[T]) TryPop() (T, bool) {
	return rb.Pop()
}

// Snapshot returns a copy of all current elements without removing them.
// This acquires a consistent view of the buffer at the time of call.
// For high-frequency use, consider using Pop directly.
func (rb *RingBuffer[T]) Snapshot() []T {
	head := rb.head.Load()
	tail := rb.tail.Load()

	n := int(tail - head)
	if n == 0 {
		return nil
	}

	result := make([]T, n)
	for i := 0; i < n; i++ {
		result[i] = rb.buffer[(head+uint64(i))&rb.mask]
	}

	return result
}

// Reset clears the buffer by resetting head and tail to zero.
// Must only be called when there are no concurrent operations.
func (rb *RingBuffer[T]) Reset() {
	rb.head.Store(0)
	rb.tail.Store(0)
}

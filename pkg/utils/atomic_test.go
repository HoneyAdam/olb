package utils

import (
	"sync"
	"testing"
	"time"
)

func TestAtomicFloat64(t *testing.T) {
	af := NewAtomicFloat64(1.5)

	// Load
	if af.Load() != 1.5 {
		t.Errorf("Load() = %f, want 1.5", af.Load())
	}

	// Store
	af.Store(2.5)
	if af.Load() != 2.5 {
		t.Errorf("Load() after Store = %f, want 2.5", af.Load())
	}

	// Add
	newVal := af.Add(0.5)
	if newVal != 3.0 {
		t.Errorf("Add() = %f, want 3.0", newVal)
	}
	if af.Load() != 3.0 {
		t.Errorf("Load() after Add = %f, want 3.0", af.Load())
	}

	// Sub
	newVal = af.Sub(1.0)
	if newVal != 2.0 {
		t.Errorf("Sub() = %f, want 2.0", newVal)
	}

	// CompareAndSwap
	if !af.CompareAndSwap(2.0, 5.0) {
		t.Error("CompareAndSwap(2.0, 5.0) should succeed")
	}
	if af.Load() != 5.0 {
		t.Errorf("Load() after CAS = %f, want 5.0", af.Load())
	}

	if af.CompareAndSwap(2.0, 10.0) {
		t.Error("CompareAndSwap(2.0, 10.0) should fail")
	}
	if af.Load() != 5.0 {
		t.Errorf("Load() should still be 5.0, got %f", af.Load())
	}
}

func TestAtomicFloat64_Concurrent(t *testing.T) {
	af := NewAtomicFloat64(0)
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Adders
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				af.Add(0.1)
			}
		}()
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = af.Load()
			}
		}()
	}

	wg.Wait()

	expected := float64(numGoroutines*iterations) * 0.1
	actual := af.Load()
	// Float math can have rounding errors
	if actual < expected-0.1 || actual > expected+0.1 {
		t.Errorf("Final value = %f, expected approximately %f", actual, expected)
	}
}

func TestAtomicDuration(t *testing.T) {
	ad := NewAtomicDuration(time.Second)

	// Load
	if ad.Load() != time.Second {
		t.Errorf("Load() = %v, want 1s", ad.Load())
	}

	// Store
	ad.Store(time.Minute)
	if ad.Load() != time.Minute {
		t.Errorf("Load() after Store = %v, want 1m", ad.Load())
	}

	// Add
	newVal := ad.Add(time.Second)
	if newVal != time.Minute+time.Second {
		t.Errorf("Add() = %v, want 1m1s", newVal)
	}

	// Sub
	newVal = ad.Sub(time.Second)
	if newVal != time.Minute {
		t.Errorf("Sub() = %v, want 1m", newVal)
	}

	// CompareAndSwap
	if !ad.CompareAndSwap(time.Minute, time.Hour) {
		t.Error("CompareAndSwap(1m, 1h) should succeed")
	}
	if ad.Load() != time.Hour {
		t.Errorf("Load() after CAS = %v, want 1h", ad.Load())
	}

	if ad.CompareAndSwap(time.Minute, time.Second) {
		t.Error("CompareAndSwap(1m, 1s) should fail")
	}
}

func TestAtomicDuration_Concurrent(t *testing.T) {
	ad := NewAtomicDuration(0)
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Adders
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ad.Add(time.Millisecond)
			}
		}()
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = ad.Load()
			}
		}()
	}

	wg.Wait()

	expected := time.Duration(numGoroutines*iterations) * time.Millisecond
	if ad.Load() != expected {
		t.Errorf("Final value = %v, want %v", ad.Load(), expected)
	}
}

func BenchmarkAtomicFloat64_Add(b *testing.B) {
	af := NewAtomicFloat64(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			af.Add(0.1)
		}
	})
}

func BenchmarkAtomicDuration_Add(b *testing.B) {
	ad := NewAtomicDuration(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ad.Add(time.Millisecond)
		}
	})
}

// TestAtomicFloat64Operations tests all AtomicFloat64 operations
func TestAtomicFloat64Operations(t *testing.T) {
	af := NewAtomicFloat64(0)

	// Initial value
	if af.Load() != 0 {
		t.Errorf("Initial Load() = %f, want 0", af.Load())
	}

	// Store
	af.Store(3.14)
	if af.Load() != 3.14 {
		t.Errorf("Load() after Store = %f, want 3.14", af.Load())
	}

	// Add
	result := af.Add(1.86)
	if result != 5.0 {
		t.Errorf("Add() = %f, want 5.0", result)
	}
	if af.Load() != 5.0 {
		t.Errorf("Load() after Add = %f, want 5.0", af.Load())
	}

	// Sub
	result = af.Sub(2.0)
	if result != 3.0 {
		t.Errorf("Sub() = %f, want 3.0", result)
	}
	if af.Load() != 3.0 {
		t.Errorf("Load() after Sub = %f, want 3.0", af.Load())
	}

	// CompareAndSwap - success
	if !af.CompareAndSwap(3.0, 10.0) {
		t.Error("CompareAndSwap(3.0, 10.0) should succeed")
	}
	if af.Load() != 10.0 {
		t.Errorf("Load() after CAS = %f, want 10.0", af.Load())
	}

	// CompareAndSwap - failure
	if af.CompareAndSwap(3.0, 20.0) {
		t.Error("CompareAndSwap(3.0, 20.0) should fail")
	}
	if af.Load() != 10.0 {
		t.Errorf("Load() should still be 10.0, got %f", af.Load())
	}

	// Negative values
	af.Store(-5.5)
	if af.Load() != -5.5 {
		t.Errorf("Load() after negative Store = %f, want -5.5", af.Load())
	}

	result = af.Add(-1.5)
	if result != -7.0 {
		t.Errorf("Add(-1.5) = %f, want -7.0", result)
	}

	// Very small values
	af.Store(1e-10)
	if af.Load() != 1e-10 {
		t.Errorf("Load() after small Store = %e, want 1e-10", af.Load())
	}

	// Very large values
	af.Store(1e10)
	if af.Load() != 1e10 {
		t.Errorf("Load() after large Store = %e, want 1e10", af.Load())
	}
}

// TestAtomicDurationOperations tests all AtomicDuration operations
func TestAtomicDurationOperations(t *testing.T) {
	ad := NewAtomicDuration(0)

	// Initial value
	if ad.Load() != 0 {
		t.Errorf("Initial Load() = %v, want 0", ad.Load())
	}

	// Store
	ad.Store(time.Second)
	if ad.Load() != time.Second {
		t.Errorf("Load() after Store = %v, want 1s", ad.Load())
	}

	// Add
	result := ad.Add(time.Millisecond)
	if result != time.Second+time.Millisecond {
		t.Errorf("Add() = %v, want 1.001s", result)
	}
	if ad.Load() != time.Second+time.Millisecond {
		t.Errorf("Load() after Add = %v, want 1.001s", ad.Load())
	}

	// Sub
	result = ad.Sub(time.Second)
	if result != time.Millisecond {
		t.Errorf("Sub() = %v, want 1ms", result)
	}
	if ad.Load() != time.Millisecond {
		t.Errorf("Load() after Sub = %v, want 1ms", ad.Load())
	}

	// CompareAndSwap - success
	if !ad.CompareAndSwap(time.Millisecond, time.Minute) {
		t.Error("CompareAndSwap(1ms, 1m) should succeed")
	}
	if ad.Load() != time.Minute {
		t.Errorf("Load() after CAS = %v, want 1m", ad.Load())
	}

	// CompareAndSwap - failure
	if ad.CompareAndSwap(time.Millisecond, time.Hour) {
		t.Error("CompareAndSwap(1ms, 1h) should fail")
	}
	if ad.Load() != time.Minute {
		t.Errorf("Load() should still be 1m, got %v", ad.Load())
	}

	// Negative duration
	ad.Store(-time.Second)
	if ad.Load() != -time.Second {
		t.Errorf("Load() after negative Store = %v, want -1s", ad.Load())
	}

	result = ad.Add(-time.Second)
	if result != -2*time.Second {
		t.Errorf("Add(-1s) = %v, want -2s", result)
	}

	// Large duration
	ad.Store(24 * time.Hour)
	if ad.Load() != 24*time.Hour {
		t.Errorf("Load() after large Store = %v, want 24h", ad.Load())
	}
}

// TestAtomicFloat64ConcurrentAccess tests concurrent AtomicFloat64 access
func TestAtomicFloat64ConcurrentAccess(t *testing.T) {
	af := NewAtomicFloat64(0)
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	// Adders
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				af.Add(0.1)
			}
		}()
	}

	// Subtractors
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				af.Sub(0.05)
			}
		}()
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = af.Load()
			}
		}()
	}

	wg.Wait()

	// Expected: (100 * 1000 * 0.1) - (100 * 1000 * 0.05) = 5000
	expected := float64(numGoroutines*iterations) * (0.1 - 0.05)
	actual := af.Load()
	tolerance := 1.0 // Allow for floating point rounding

	if actual < expected-tolerance || actual > expected+tolerance {
		t.Errorf("Final value = %f, expected approximately %f", actual, expected)
	}
}

// TestAtomicDurationConcurrentAccess tests concurrent AtomicDuration access
func TestAtomicDurationConcurrentAccess(t *testing.T) {
	ad := NewAtomicDuration(0)
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	// Adders
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ad.Add(time.Millisecond)
			}
		}()
	}

	// Subtractors
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/2; j++ {
				ad.Sub(2 * time.Millisecond)
			}
		}()
	}

	// Readers and CASers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				val := ad.Load()
				if j%100 == 0 {
					// Occasionally try CAS
					ad.CompareAndSwap(val, val)
				}
			}
		}(i)
	}

	wg.Wait()

	// Expected: (100 * 1000 * 1ms) - (100 * 500 * 2ms) = 0
	expected := time.Duration(numGoroutines*iterations)*time.Millisecond -
		time.Duration(numGoroutines*(iterations/2))*2*time.Millisecond

	if ad.Load() != expected {
		t.Errorf("Final value = %v, want %v", ad.Load(), expected)
	}
}

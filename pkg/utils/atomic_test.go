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

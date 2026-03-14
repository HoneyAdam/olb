package utils

import (
	"bytes"
	"sync"
	"testing"
)

func TestBufferPool_GetPut(t *testing.T) {
	pool := NewBufferPool()

	tests := []struct {
		name     string
		reqSize  int
		wantCap  int
		wantPool bool // should come from pool
	}{
		{"tiny", 100, SmallBufferSize, true},
		{"small_exact", SmallBufferSize, SmallBufferSize, true},
		{"small_plus", SmallBufferSize + 100, MediumBufferSize, true},
		{"medium_exact", MediumBufferSize, MediumBufferSize, true},
		{"medium_plus", MediumBufferSize + 100, LargeBufferSize, true},
		{"large_exact", LargeBufferSize, LargeBufferSize, true},
		{"large_plus", LargeBufferSize + 100, XLargeBufferSize, true},
		{"xlarge_exact", XLargeBufferSize, XLargeBufferSize, true},
		{"xlarge_plus", XLargeBufferSize + 100, XLargeBufferSize + 100, false}, // oversized, not pooled
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := pool.Get(tt.reqSize)

			// Check length is exactly what was requested
			if len(buf) != tt.reqSize {
				t.Errorf("Get(%d) length = %d, want %d", tt.reqSize, len(buf), tt.reqSize)
			}

			// Check capacity is from correct tier
			if cap(buf) != tt.wantCap {
				t.Errorf("Get(%d) capacity = %d, want %d", tt.reqSize, cap(buf), tt.wantCap)
			}

			// Write some data
			for i := range buf {
				buf[i] = byte(i % 256)
			}

			// Return to pool
			pool.Put(buf)

			// Get again and verify zeroed (security check)
			buf2 := pool.Get(tt.reqSize)
			if len(buf2) != tt.reqSize {
				t.Errorf("Get second time length = %d, want %d", len(buf2), tt.reqSize)
			}

			// Buffer should be zeroed
			for i, b := range buf2 {
				if b != 0 {
					t.Errorf("Buffer not zeroed at index %d: got %d", i, b)
					break
				}
			}

			pool.Put(buf2)
		})
	}
}

func TestBufferPool_GetZeroSize(t *testing.T) {
	pool := NewBufferPool()

	buf := pool.Get(0)
	if buf != nil {
		t.Errorf("Get(0) = %v, want nil", buf)
	}

	buf = pool.Get(-1)
	if buf != nil {
		t.Errorf("Get(-1) = %v, want nil", buf)
	}
}

func TestBufferPool_PutNil(t *testing.T) {
	pool := NewBufferPool()

	// Should not panic
	pool.Put(nil)
	pool.Put([]byte{})
}

func TestBufferPool_PutOversized(t *testing.T) {
	pool := NewBufferPool()

	// Oversized buffer should not be returned to pool (no panic)
	buf := make([]byte, maxBufferSize+1000)
	pool.Put(buf)
}

func TestBufferPool_PutWrongSize(t *testing.T) {
	pool := NewBufferPool()

	// Buffer with non-standard capacity should not be pooled
	buf := make([]byte, 100, 5000) // 5KB is not a tier
	pool.Put(buf)
}

func TestBufferPool_Concurrent(t *testing.T) {
	pool := NewBufferPool()
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Request various sizes
				size := (j % XLargeBufferSize) + 1
				buf := pool.Get(size)

				// Write data
				for k := range buf {
					buf[k] = byte(id)
				}

				// Return to pool
				pool.Put(buf)
			}
		}(i)
	}

	wg.Wait()
}

func TestBufferPool_Copy(t *testing.T) {
	pool := NewBufferPool()

	original := []byte("hello, world!")
	copy1 := pool.Copy(original)

	if !bytes.Equal(copy1, original) {
		t.Errorf("Copy = %v, want %v", copy1, original)
	}

	// Modifying copy should not affect original
	copy1[0] = 'X'
	if !bytes.Equal(original, []byte("hello, world!")) {
		t.Error("Copy modification affected original")
	}

	pool.Put(copy1)

	// Empty copy
	empty := pool.Copy(nil)
	if empty != nil {
		t.Errorf("Copy(nil) = %v, want nil", empty)
	}

	empty = pool.Copy([]byte{})
	if empty != nil {
		t.Errorf("Copy(empty) = %v, want nil", empty)
	}
}

func TestBufferPool_Grow(t *testing.T) {
	pool := NewBufferPool()

	// Grow within capacity
	buf := pool.Get(100)
	copy(buf, []byte("test data"))
	grown := pool.Grow(buf, 50)

	if cap(grown) != SmallBufferSize {
		t.Errorf("Grow within capacity: cap = %d, want %d", cap(grown), SmallBufferSize)
	}
	if string(grown[:9]) != "test data" {
		t.Error("Grow within capacity: data lost")
	}

	// Return
	pool.Put(grown)

	// Grow beyond capacity
	buf = pool.Get(100)
	copy(buf, []byte("test data"))
	grown = pool.Grow(buf, 2000)

	if cap(grown) != MediumBufferSize {
		t.Errorf("Grow beyond capacity: cap = %d, want %d", cap(grown), MediumBufferSize)
	}
	if string(grown[:9]) != "test data" {
		t.Error("Grow beyond capacity: data not copied")
	}

	pool.Put(grown)
}

func TestDefaultBufferPool(t *testing.T) {
	// Test global functions
	buf := Get(100)
	if len(buf) != 100 {
		t.Errorf("Get(100) length = %d, want 100", len(buf))
	}
	Put(buf)

	// Copy
	original := []byte("test")
	copy1 := Copy(original)
	if !bytes.Equal(copy1, original) {
		t.Error("Copy failed")
	}
	Put(copy1)

	// Grow
	buf = Get(50)
	copy(buf, []byte("grow"))
	grown := Grow(buf, 100)
	if string(grown[:4]) != "grow" {
		t.Error("Grow failed")
	}
	Put(grown)
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(1024)
		pool.Put(buf)
	}
}

func BenchmarkBufferPool_GetPutParallel(b *testing.B) {
	pool := NewBufferPool()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(1024)
			pool.Put(buf)
		}
	})
}

func BenchmarkBufferPool_NoPool(b *testing.B) {
	// Benchmark without pooling (allocation every time)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		_ = buf
	}
}

// TestBufferPool_GetPutAllSizes tests Get/Put for all buffer size tiers
func TestBufferPool_GetPutAllSizes(t *testing.T) {
	pool := NewBufferPool()

	sizes := []struct {
		name string
		size int
		cap  int
	}{
		{"tiny", 100, SmallBufferSize},
		{"small_exact", SmallBufferSize, SmallBufferSize},
		{"small_boundary", SmallBufferSize - 1, SmallBufferSize},
		{"medium_exact", MediumBufferSize, MediumBufferSize},
		{"medium_boundary", MediumBufferSize - 1, MediumBufferSize},
		{"large_exact", LargeBufferSize, LargeBufferSize},
		{"large_boundary", LargeBufferSize - 1, LargeBufferSize},
		{"xlarge_exact", XLargeBufferSize, XLargeBufferSize},
		{"xlarge_boundary", XLargeBufferSize - 1, XLargeBufferSize},
	}

	for _, tt := range sizes {
		t.Run(tt.name, func(t *testing.T) {
			buf := pool.Get(tt.size)
			if len(buf) != tt.size {
				t.Errorf("Get(%d) length = %d, want %d", tt.size, len(buf), tt.size)
			}
			if cap(buf) != tt.cap {
				t.Errorf("Get(%d) capacity = %d, want %d", tt.size, cap(buf), tt.cap)
			}
			pool.Put(buf)
		})
	}
}

// TestBufferPool_PutWrongSizeDetailed tests putting buffers with wrong sizes
func TestBufferPool_PutWrongSizeDetailed(t *testing.T) {
	pool := NewBufferPool()

	// Create buffers with non-standard capacities
	wrongSizes := []int{
		100,
		500,
		1000,
		5000,
		10000,
		50000,
		100000,
	}

	for _, size := range wrongSizes {
		// These should not panic, just not be pooled
		buf := make([]byte, 100, size)
		pool.Put(buf)
	}

	// nil and empty should not panic
	pool.Put(nil)
	pool.Put([]byte{})
}

// TestBufferPool_ConcurrentAccess tests concurrent Get/Put operations
func TestBufferPool_ConcurrentAccess(t *testing.T) {
	pool := NewBufferPool()
	const numGoroutines = 50
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine gets and puts buffers of various sizes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Cycle through different sizes
				size := (j % (XLargeBufferSize + 100)) + 1
				buf := pool.Get(size)

				// Write some data to verify buffer is usable
				for k := 0; k < len(buf) && k < 100; k++ {
					buf[k] = byte(id % 256)
				}

				pool.Put(buf)
			}
		}(i)
	}

	wg.Wait()
}

// TestBufferPool_OversizedAllocation tests oversized buffer allocation
func TestBufferPool_OversizedAllocation(t *testing.T) {
	pool := NewBufferPool()

	// Request buffer larger than maxBufferSize
	oversized := XLargeBufferSize + 1000
	buf := pool.Get(oversized)

	if len(buf) != oversized {
		t.Errorf("Oversized buffer length = %d, want %d", len(buf), oversized)
	}

	// Oversized buffers should have exact capacity, not pooled size
	if cap(buf) != oversized {
		t.Errorf("Oversized buffer capacity = %d, want %d", cap(buf), oversized)
	}

	// Writing should work
	for i := range buf {
		buf[i] = byte(i % 256)
	}

	// Putting oversized buffer should not panic (but won't be pooled)
	pool.Put(buf)
}

// TestBufferPool_BufferReuse tests that buffers are actually reused
func TestBufferPool_BufferReuse(t *testing.T) {
	pool := NewBufferPool()

	// Get a buffer and note its pointer
	buf1 := pool.Get(100)
	ptr1 := &buf1[0]
	pool.Put(buf1)

	// Get another buffer of same size tier
	buf2 := pool.Get(100)
	ptr2 := &buf2[0]

	// The same underlying array should be reused
	if ptr1 != ptr2 {
		t.Log("Note: Buffer was not reused (this is OK, sync.Pool doesn't guarantee reuse)")
	}

	pool.Put(buf2)
}

// TestBufferPool_DataClearing tests that buffers are zeroed before reuse
func TestBufferPool_DataClearing(t *testing.T) {
	pool := NewBufferPool()

	// Get buffer and fill with non-zero data
	buf := pool.Get(100)
	for i := range buf {
		buf[i] = 0xFF
	}
	pool.Put(buf)

	// Get buffer again - should be zeroed
	buf2 := pool.Get(100)
	for i, b := range buf2 {
		if b != 0 {
			t.Errorf("Buffer not zeroed at index %d: got %d", i, b)
			break
		}
	}
	pool.Put(buf2)
}

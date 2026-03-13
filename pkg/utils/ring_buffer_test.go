package utils

import (
	"sync"
	"testing"
)

func TestNewRingBuffer(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int
		wantCap     int
		wantErr     bool
	}{
		{"exact power of 2", 1024, 1024, false},
		{"round up 1000", 1000, 1024, false},
		{"round up 3", 3, 4, false},
		{"capacity 1", 1, 1, false},
		{"capacity 0", 0, 0, true},
		{"negative capacity", -1, 0, true},
		{"large capacity", 65535, 65536, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := NewRingBuffer[int](tt.capacity)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRingBuffer(%d) error = %v, wantErr %v", tt.capacity, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if rb.Capacity() != tt.wantCap {
				t.Errorf("Capacity() = %d, want %d", rb.Capacity(), tt.wantCap)
			}
		})
	}
}

func TestRingBuffer_PushPop(t *testing.T) {
	rb := MustNewRingBuffer[int](4)

	// Push some values
	for i := 1; i <= 3; i++ {
		if !rb.Push(i) {
			t.Errorf("Push(%d) failed, expected success", i)
		}
	}

	// Buffer should not be empty
	if rb.IsEmpty() {
		t.Error("IsEmpty() = true, expected false")
	}

	// Buffer should not be full (3/4)
	if rb.IsFull() {
		t.Error("IsFull() = true, expected false")
	}

	// Pop values
	for i := 1; i <= 3; i++ {
		val, ok := rb.Pop()
		if !ok {
			t.Errorf("Pop() failed, expected success")
		}
		if val != i {
			t.Errorf("Pop() = %d, want %d", val, i)
		}
	}

	// Buffer should be empty now
	if !rb.IsEmpty() {
		t.Error("IsEmpty() = false after popping all, expected true")
	}

	// Pop from empty should fail
	_, ok := rb.Pop()
	if ok {
		t.Error("Pop() from empty succeeded, expected failure")
	}
}

func TestRingBuffer_Full(t *testing.T) {
	rb := MustNewRingBuffer[int](4)

	// Fill completely
	for i := 0; i < 4; i++ {
		if !rb.Push(i) {
			t.Errorf("Push(%d) failed", i)
		}
	}

	// Buffer should be full
	if !rb.IsFull() {
		t.Error("IsFull() = false, expected true")
	}

	// Push to full should fail
	if rb.Push(99) {
		t.Error("Push to full buffer succeeded, expected failure")
	}

	// Pop one
	val, ok := rb.Pop()
	if !ok || val != 0 {
		t.Errorf("Pop() = %d, %v, want 0, true", val, ok)
	}

	// Now we can push one more
	if !rb.Push(99) {
		t.Error("Push after Pop failed, expected success")
	}
}

func TestRingBuffer_Len(t *testing.T) {
	rb := MustNewRingBuffer[int](8)

	if rb.Len() != 0 {
		t.Errorf("Len() = %d, want 0", rb.Len())
	}

	rb.Push(1)
	rb.Push(2)

	if rb.Len() != 2 {
		t.Errorf("Len() = %d, want 2", rb.Len())
	}

	rb.Pop()

	if rb.Len() != 1 {
		t.Errorf("Len() = %d, want 1", rb.Len())
	}
}

func TestRingBuffer_WrapAround(t *testing.T) {
	rb := MustNewRingBuffer[int](4)

	// Fill, drain, fill again to test wrap-around
	for round := 0; round < 3; round++ {
		// Push
		for i := 0; i < 4; i++ {
			if !rb.Push(round*10 + i) {
				t.Fatalf("Push failed at round %d, i %d", round, i)
			}
		}

		// Pop and verify
		for i := 0; i < 4; i++ {
			val, ok := rb.Pop()
			if !ok {
				t.Fatalf("Pop failed at round %d, i %d", round, i)
			}
			expected := round*10 + i
			if val != expected {
				t.Errorf("Pop() = %d, want %d", val, expected)
			}
		}
	}
}

func TestRingBuffer_Snapshot(t *testing.T) {
	rb := MustNewRingBuffer[int](8)

	// Empty snapshot
	snap := rb.Snapshot()
	if snap != nil && len(snap) != 0 {
		t.Errorf("Snapshot of empty = %v, want nil or empty", snap)
	}

	// Push values
	for i := 1; i <= 5; i++ {
		rb.Push(i)
	}

	// Get snapshot
	snap = rb.Snapshot()
	if len(snap) != 5 {
		t.Errorf("Snapshot len = %d, want 5", len(snap))
	}

	for i, v := range snap {
		if v != i+1 {
			t.Errorf("Snapshot[%d] = %d, want %d", i, v, i+1)
		}
	}

	// Snapshot doesn't modify buffer
	if rb.Len() != 5 {
		t.Errorf("Len after snapshot = %d, want 5", rb.Len())
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	rb := MustNewRingBuffer[int](4)

	// Push some values
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)

	// Reset
	rb.Reset()

	// Should be empty
	if !rb.IsEmpty() {
		t.Error("IsEmpty() = false after Reset, expected true")
	}

	// Can push again
	if !rb.Push(99) {
		t.Error("Push after Reset failed")
	}
}

func TestRingBuffer_Generic(t *testing.T) {
	// Test with string type
	rb, _ := NewRingBuffer[string](4)
	rb.Push("hello")
	rb.Push("world")

	val, ok := rb.Pop()
	if !ok || val != "hello" {
		t.Errorf("Pop() = %s, %v, want hello, true", val, ok)
	}

	val, ok = rb.Pop()
	if !ok || val != "world" {
		t.Errorf("Pop() = %s, %v, want world, true", val, ok)
	}

	// Test with struct type
	type Point struct {
		X, Y int
	}
	rb2, _ := NewRingBuffer[Point](4)
	rb2.Push(Point{1, 2})
	rb2.Push(Point{3, 4})

	p, ok := rb2.Pop()
	if !ok || p.X != 1 || p.Y != 2 {
		t.Errorf("Pop() = %+v, %v, want {1 2}, true", p, ok)
	}
}

func TestRingBuffer_ConcurrentSPSC(t *testing.T) {
	rb := MustNewRingBuffer[int](1024)
	const count = 10000

	var wg sync.WaitGroup
	wg.Add(2)

	// Producer
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			for !rb.Push(i) {
				// Spin until space available
			}
		}
	}()

	// Consumer
	var sum int
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			val, ok := rb.Pop()
			for !ok {
				val, ok = rb.Pop()
			}
			sum += val
		}
	}()

	wg.Wait()

	// Sum of 0 to count-1 = count*(count-1)/2
	expected := count * (count - 1) / 2
	if sum != expected {
		t.Errorf("Sum = %d, want %d", sum, expected)
	}
}

func BenchmarkRingBuffer_PushPop(b *testing.B) {
	rb := MustNewRingBuffer[int](1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Push(i)
		rb.Pop()
	}
}

func BenchmarkRingBuffer_PushPopContended(b *testing.B) {
	rb := MustNewRingBuffer[int](1024)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				rb.Push(i)
			} else {
				rb.Pop()
			}
			i++
		}
	})
}

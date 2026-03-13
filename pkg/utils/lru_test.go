package utils

import (
	"sync"
	"testing"
	"time"
)

func TestNewLRU(t *testing.T) {
	// Valid capacity
	lru, err := NewLRU[string, int](10)
	if err != nil {
		t.Fatalf("NewLRU(10) error = %v", err)
	}
	if lru.Capacity() != 10 {
		t.Errorf("Capacity() = %d, want 10", lru.Capacity())
	}

	// Invalid capacity
	_, err = NewLRU[string, int](0)
	if err == nil {
		t.Error("NewLRU(0) should error")
	}

	_, err = NewLRU[string, int](-1)
	if err == nil {
		t.Error("NewLRU(-1) should error")
	}
}

func TestLRU_Basic(t *testing.T) {
	lru := MustNewLRU[string, int](3)

	// Put and Get
	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	if lru.Len() != 3 {
		t.Errorf("Len() = %d, want 3", lru.Len())
	}

	val, ok := lru.Get("a")
	if !ok || val != 1 {
		t.Errorf("Get(a) = %d, %v, want 1, true", val, ok)
	}

	val, ok = lru.Get("b")
	if !ok || val != 2 {
		t.Errorf("Get(b) = %d, %v, want 2, true", val, ok)
	}

	val, ok = lru.Get("c")
	if !ok || val != 3 {
		t.Errorf("Get(c) = %d, %v, want 3, true", val, ok)
	}

	// Non-existent key
	val, ok = lru.Get("d")
	if ok {
		t.Errorf("Get(d) = %d, %v, want zero, false", val, ok)
	}
}

func TestLRU_Eviction(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	// Fill cache
	lru.Put("a", 1)
	lru.Put("b", 2)

	// Add third, should evict 'a' (LRU)
	lru.Put("c", 3)

	// 'a' should be evicted
	_, ok := lru.Get("a")
	if ok {
		t.Error("Get(a) should fail, 'a' should be evicted")
	}

	// 'b' and 'c' should exist
	val, ok := lru.Get("b")
	if !ok || val != 2 {
		t.Errorf("Get(b) = %d, %v, want 2, true", val, ok)
	}

	val, ok = lru.Get("c")
	if !ok || val != 3 {
		t.Errorf("Get(c) = %d, %v, want 3, true", val, ok)
	}
}

func TestLRU_UpdateOrder(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	lru.Put("a", 1)
	lru.Put("b", 2)

	// Access 'a' to make it most recently used
	lru.Get("a")

	// Add 'c', should evict 'b' (now LRU)
	lru.Put("c", 3)

	_, ok := lru.Get("b")
	if ok {
		t.Error("Get(b) should fail, 'b' should be evicted")
	}

	val, ok := lru.Get("a")
	if !ok || val != 1 {
		t.Errorf("Get(a) = %d, %v, want 1, true", val, ok)
	}
}

func TestLRU_UpdateValue(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	lru.Put("a", 1)
	lru.Put("a", 100) // Update

	val, ok := lru.Get("a")
	if !ok || val != 100 {
		t.Errorf("Get(a) = %d, %v, want 100, true", val, ok)
	}

	if lru.Len() != 1 {
		t.Errorf("Len() = %d, want 1", lru.Len())
	}
}

func TestLRU_TTL(t *testing.T) {
	lru := MustNewLRU[string, int](10)

	// Put with short TTL
	lru.PutWithTTL("a", 1, 50*time.Millisecond)
	lru.Put("b", 2) // No TTL

	// Both should exist immediately
	val, ok := lru.Get("a")
	if !ok || val != 1 {
		t.Errorf("Get(a) immediate = %d, %v, want 1, true", val, ok)
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// 'a' should be expired
	_, ok = lru.Get("a")
	if ok {
		t.Error("Get(a) after TTL should fail")
	}

	// 'b' should still exist
	val, ok = lru.Get("b")
	if !ok || val != 2 {
		t.Errorf("Get(b) = %d, %v, want 2, true", val, ok)
	}
}

func TestLRU_Contains(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	lru.Put("a", 1)
	lru.Put("b", 2)

	if !lru.Contains("a") {
		t.Error("Contains(a) should be true")
	}

	if !lru.Contains("b") {
		t.Error("Contains(b) should be true")
	}

	// Add 'c', should evict 'a' (LRU since we added a then b)
	lru.Put("c", 3)

	if lru.Contains("a") {
		t.Error("Contains(a) should be false after eviction")
	}

	if !lru.Contains("b") {
		t.Error("Contains(b) should still be true")
	}

	if !lru.Contains("c") {
		t.Error("Contains(c) should be true")
	}
}

func TestLRU_Peek(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	lru.Put("a", 1)
	lru.Put("b", 2)

	// Peek should not update order
	val, ok := lru.Peek("a")
	if !ok || val != 1 {
		t.Errorf("Peek(a) = %d, %v, want 1, true", val, ok)
	}

	// Add 'c', should evict 'a' (still LRU because Peek doesn't update)
	lru.Put("c", 3)

	_, ok = lru.Get("a")
	if ok {
		t.Error("Get(a) should fail, Peek should not update order")
	}
}

func TestLRU_Delete(t *testing.T) {
	lru := MustNewLRU[string, int](10)

	lru.Put("a", 1)

	if !lru.Delete("a") {
		t.Error("Delete(a) should return true")
	}

	if lru.Delete("a") {
		t.Error("Delete(a) again should return false")
	}

	if lru.Len() != 0 {
		t.Errorf("Len() = %d, want 0", lru.Len())
	}
}

func TestLRU_Clear(t *testing.T) {
	lru := MustNewLRU[string, int](10)

	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	lru.Clear()

	if lru.Len() != 0 {
		t.Errorf("Len() after Clear = %d, want 0", lru.Len())
	}

	_, ok := lru.Get("a")
	if ok {
		t.Error("Get(a) after Clear should fail")
	}
}

func TestLRU_Keys(t *testing.T) {
	lru := MustNewLRU[string, int](3)

	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	// Access 'a' to make it MRU
	lru.Get("a")

	keys := lru.Keys()
	// Should be in MRU order: a, c, b
	expected := []string{"a", "c", "b"}

	if len(keys) != 3 {
		t.Fatalf("Keys() len = %d, want 3", len(keys))
	}

	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Keys()[%d] = %s, want %s", i, k, expected[i])
		}
	}
}

func TestLRU_Values(t *testing.T) {
	lru := MustNewLRU[string, int](3)

	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	lru.Get("a") // Make 'a' MRU

	values := lru.Values()
	// Should be in MRU order: 1, 3, 2
	expected := []int{1, 3, 2}

	if len(values) != 3 {
		t.Fatalf("Values() len = %d, want 3", len(values))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Errorf("Values()[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestLRU_Resize(t *testing.T) {
	lru := MustNewLRU[string, int](3)

	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	// Resize down to 2
	evicted := lru.Resize(2)
	if evicted != 1 {
		t.Errorf("Resize(2) evicted = %d, want 1", evicted)
	}

	if lru.Capacity() != 2 {
		t.Errorf("Capacity() = %d, want 2", lru.Capacity())
	}

	if lru.Len() != 2 {
		t.Errorf("Len() = %d, want 2", lru.Len())
	}

	// Resize up to 5
	evicted = lru.Resize(5)
	if evicted != 0 {
		t.Errorf("Resize(5) evicted = %d, want 0", evicted)
	}

	// Can add more now
	lru.Put("d", 4)
	lru.Put("e", 5)

	if lru.Len() != 4 {
		t.Errorf("Len() = %d, want 4", lru.Len())
	}
}

func TestLRU_OnEvict(t *testing.T) {
	lru := MustNewLRU[string, int](2)

	var evictedKeys []string
	var evictedValues []int

	lru.SetOnEvict(func(key string, value int) {
		evictedKeys = append(evictedKeys, key)
		evictedValues = append(evictedValues, value)
	})

	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3) // Should evict 'a'

	if len(evictedKeys) != 1 || evictedKeys[0] != "a" {
		t.Errorf("Evicted key = %v, want ['a']", evictedKeys)
	}

	if len(evictedValues) != 1 || evictedValues[0] != 1 {
		t.Errorf("Evicted value = %v, want [1]", evictedValues)
	}

	lru.Delete("b") // Should trigger eviction callback

	if len(evictedKeys) != 2 || evictedKeys[1] != "b" {
		t.Errorf("Evicted keys after Delete = %v, want ['a', 'b']", evictedKeys)
	}
}

func TestLRU_Concurrent(t *testing.T) {
	lru := MustNewLRU[int, int](100)
	const numGoroutines = 10
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := (id*iterations + j) % 200 // Some overlap
				lru.Put(key, key*10)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := j % 200
				lru.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Cache should not exceed capacity
	if lru.Len() > lru.Capacity() {
		t.Errorf("Len() = %d > Capacity() = %d", lru.Len(), lru.Capacity())
	}
}

func BenchmarkLRU_Put(b *testing.B) {
	lru := MustNewLRU[int, int](1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Put(i%10000, i)
	}
}

func BenchmarkLRU_Get(b *testing.B) {
	lru := MustNewLRU[int, int](1000)
	for i := 0; i < 1000; i++ {
		lru.Put(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Get(i % 1000)
	}
}

func BenchmarkLRU_PutGet(b *testing.B) {
	lru := MustNewLRU[int, int](1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Put(i, i)
		lru.Get(i)
	}
}

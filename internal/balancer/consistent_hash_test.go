package balancer

import (
	"fmt"
	"hash/crc32"
	"math"
	"sync"
	"testing"

	"github.com/openloadbalancer/olb/internal/backend"
)

// TestConsistentRouting tests that the same key always routes to the same backend.
func TestConsistentRouting(t *testing.T) {
	ch := NewConsistentHash(150)

	// Create backends
	b1 := backend.NewBackend("backend1", "10.0.0.1:8080")
	b2 := backend.NewBackend("backend2", "10.0.0.2:8080")
	b3 := backend.NewBackend("backend3", "10.0.0.3:8080")

	ch.Add(b1)
	ch.Add(b2)
	ch.Add(b3)

	// Test that the same key always routes to the same backend
	testKeys := []string{"key1", "key2", "key3", "user:123", "/api/v1/users", "10.0.0.1"}

	for _, key := range testKeys {
		// Get the hash for this key
		hash := ch.hashKey(key)

		// Find which backend this routes to (multiple times)
		var results []string
		for i := 0; i < 10; i++ {
			backendID := ch.getNode(hash)
			results = append(results, backendID)
		}

		// All results should be the same
		first := results[0]
		for i, r := range results {
			if r != first {
				t.Errorf("Key %q: inconsistent routing at iteration %d: got %q, expected %q", key, i, r, first)
			}
		}
	}
}

// TestDistributionUniformity tests that keys are distributed uniformly across backends.
func TestDistributionUniformity(t *testing.T) {
	ch := NewConsistentHash(150)

	// Create 5 backends
	backends := make([]*backend.Backend, 5)
	for i := 0; i < 5; i++ {
		backends[i] = backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(backends[i])
	}

	// Generate many keys and count distribution
	counts := make(map[string]int)
	numKeys := 10000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		hash := ch.hashKey(key)
		backendID := ch.getNode(hash)
		counts[backendID]++
	}

	// Check that all backends received some traffic
	if len(counts) != 5 {
		t.Errorf("Expected 5 backends to receive traffic, got %d", len(counts))
	}

	// Calculate expected count per backend
	expected := float64(numKeys) / 5.0

	// Check that distribution is reasonably uniform (within 30% of expected)
	for backendID, count := range counts {
		diff := math.Abs(float64(count) - expected)
		percentage := diff / expected * 100
		if percentage > 30 {
			t.Errorf("Backend %s: count %d deviates too much from expected %.0f (%.1f%%)",
				backendID, count, expected, percentage)
		}
	}

	t.Logf("Distribution: %v", counts)
}

// TestMinimalRedistribution tests that adding/removing backends causes minimal key movement.
func TestMinimalRedistribution(t *testing.T) {
	// Initial setup with 4 backends
	ch := NewConsistentHash(150)
	backends := make([]*backend.Backend, 4)
	for i := 0; i < 4; i++ {
		backends[i] = backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(backends[i])
	}

	// Map keys to backends
	numKeys := 10000
	keyMapping := make(map[string]string)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		hash := ch.hashKey(key)
		keyMapping[key] = ch.getNode(hash)
	}

	// Add a new backend
	newBackend := backend.NewBackend("backend4", "10.0.0.4:8080")
	ch.Add(newBackend)

	// Count how many keys moved
	moved := 0
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		hash := ch.hashKey(key)
		newBackendID := ch.getNode(hash)
		if newBackendID != keyMapping[key] {
			moved++
		}
	}

	// Ideally, only ~20% of keys should move (1/5 of total)
	// Allow some tolerance: should be less than 35%
	movePercentage := float64(moved) / float64(numKeys) * 100
	if movePercentage > 35 {
		t.Errorf("Too many keys moved when adding backend: %.1f%% (expected ~20%%)", movePercentage)
	}
	t.Logf("Keys moved when adding backend: %d (%.1f%%)", moved, movePercentage)

	// Now remove the backend we just added
	ch.Remove("backend4")

	// Keys should move back to their original backends
	movedBack := 0
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		hash := ch.hashKey(key)
		newBackendID := ch.getNode(hash)
		if newBackendID != keyMapping[key] {
			movedBack++
		}
	}

	// After removing the added backend, almost all keys should be back to original
	// Allow small tolerance due to hash collisions
	if movedBack > numKeys/100 { // 1% tolerance
		t.Errorf("Keys didn't return to original backends after removal: %d still moved", movedBack)
	}
}

// TestDifferentVnodeCounts tests behavior with different virtual node counts.
func TestDifferentVnodeCounts(t *testing.T) {
	vnodeCounts := []int{10, 50, 100, 150, 300}

	for _, vnodes := range vnodeCounts {
		t.Run(fmt.Sprintf("vnodes_%d", vnodes), func(t *testing.T) {
			ch := NewConsistentHash(vnodes)

			// Add backends
			for i := 0; i < 3; i++ {
				b := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
				ch.Add(b)
			}

			// Check ring size
			expectedSize := 3 * vnodes
			if ch.RingSize() != expectedSize {
				t.Errorf("Expected ring size %d, got %d", expectedSize, ch.RingSize())
			}

			// Test basic routing still works
			hash := ch.hashKey("test-key")
			backendID := ch.getNode(hash)
			if backendID == "" {
				t.Error("Expected a backend, got empty string")
			}
		})
	}
}

// TestEmptyRing tests behavior with an empty ring.
func TestEmptyRing(t *testing.T) {
	ch := NewConsistentHash(150)

	// Test getNode on empty ring
	backendID := ch.getNode(12345)
	if backendID != "" {
		t.Errorf("Expected empty string for empty ring, got %q", backendID)
	}

	// Test Next with no backends
	result := ch.Next([]*backend.Backend{})
	if result != nil {
		t.Errorf("Expected nil for empty backends, got %v", result)
	}

	// Test Next with backends but empty ring
	b := backend.NewBackend("backend1", "10.0.0.1:8080")
	result = ch.Next([]*backend.Backend{b})
	if result != nil {
		t.Errorf("Expected nil for empty ring, got %v", result)
	}
}

// TestConcurrentAccess tests thread safety.
func TestConcurrentAccess(t *testing.T) {
	ch := NewConsistentHash(150)

	// Add initial backends
	for i := 0; i < 3; i++ {
		b := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(b)
	}

	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	// Get backends list for Next calls
	backends := make([]*backend.Backend, 3)
	for i := 0; i < 3; i++ {
		backends[i] = backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = ch.Next(backends)
			}
		}(i)
	}

	// Concurrent adds/removes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				backendID := fmt.Sprintf("dynamic%d", id)
				b := backend.NewBackend(backendID, fmt.Sprintf("10.0.1.%d:8080", id))
				ch.Add(b)
				ch.Remove(backendID)
			}
		}(i)
	}

	wg.Wait()

	// Verify ring is still consistent
	if ch.BackendCount() != 3 {
		t.Errorf("Expected 3 backends after concurrent operations, got %d", ch.BackendCount())
	}
}

// TestAddRemoveUpdate tests the Add, Remove, and Update methods.
func TestAddRemoveUpdate(t *testing.T) {
	ch := NewConsistentHash(150)

	// Test Add
	b1 := backend.NewBackend("backend1", "10.0.0.1:8080")
	ch.Add(b1)

	if ch.BackendCount() != 1 {
		t.Errorf("Expected 1 backend, got %d", ch.BackendCount())
	}

	if ch.RingSize() != 150 {
		t.Errorf("Expected 150 ring nodes, got %d", ch.RingSize())
	}

	// Add another backend
	b2 := backend.NewBackend("backend2", "10.0.0.2:8080")
	ch.Add(b2)

	if ch.BackendCount() != 2 {
		t.Errorf("Expected 2 backends, got %d", ch.BackendCount())
	}

	if ch.RingSize() != 300 {
		t.Errorf("Expected 300 ring nodes, got %d", ch.RingSize())
	}

	// Test Remove
	ch.Remove("backend1")

	if ch.BackendCount() != 1 {
		t.Errorf("Expected 1 backend after removal, got %d", ch.BackendCount())
	}

	if ch.RingSize() != 150 {
		t.Errorf("Expected 150 ring nodes after removal, got %d", ch.RingSize())
	}

	// Test Update (should not change ring structure)
	b2Updated := backend.NewBackend("backend2", "10.0.0.2:8081")
	ch.Update(b2Updated)

	if ch.BackendCount() != 1 {
		t.Errorf("Expected 1 backend after update, got %d", ch.BackendCount())
	}

	if ch.RingSize() != 150 {
		t.Errorf("Expected 150 ring nodes after update, got %d", ch.RingSize())
	}

	// Test removing non-existent backend
	ch.Remove("nonexistent")

	if ch.BackendCount() != 1 {
		t.Errorf("Expected 1 backend after removing non-existent, got %d", ch.BackendCount())
	}
}

// TestSetVirtualNodes tests changing the number of virtual nodes.
func TestSetVirtualNodes(t *testing.T) {
	ch := NewConsistentHash(50)

	// Add backends
	for i := 0; i < 3; i++ {
		b := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(b)
	}

	if ch.RingSize() != 150 {
		t.Errorf("Expected 150 ring nodes, got %d", ch.RingSize())
	}

	// Change virtual nodes
	ch.SetVirtualNodes(100)

	if ch.GetVirtualNodes() != 100 {
		t.Errorf("Expected 100 virtual nodes, got %d", ch.GetVirtualNodes())
	}

	if ch.RingSize() != 300 {
		t.Errorf("Expected 300 ring nodes after resize, got %d", ch.RingSize())
	}

	// Verify routing still works
	hash := ch.hashKey("test")
	backendID := ch.getNode(hash)
	if backendID == "" {
		t.Error("Expected a backend after resize, got empty string")
	}
}

// TestHashKey tests the hash function.
func TestHashKey(t *testing.T) {
	ch := NewConsistentHash(150)

	// Test that hashKey produces consistent results
	hash1 := ch.hashKey("test-key")
	hash2 := ch.hashKey("test-key")
	if hash1 != hash2 {
		t.Errorf("Hash function not consistent: %d != %d", hash1, hash2)
	}

	// Test that different keys produce different hashes (usually)
	hash3 := ch.hashKey("different-key")
	if hash1 == hash3 {
		t.Log("Warning: hash collision detected (rare but possible)")
	}

	// Verify it matches CRC32
	expected := crc32.ChecksumIEEE([]byte("test-key"))
	if hash1 != expected {
		t.Errorf("Hash mismatch: got %d, expected %d", hash1, expected)
	}
}

// TestBinarySearch tests the binary search implementation.
func TestBinarySearch(t *testing.T) {
	ch := NewConsistentHash(150)

	// Add a backend to populate the ring
	b := backend.NewBackend("backend1", "10.0.0.1:8080")
	ch.Add(b)

	// The ring should have nodes now
	if len(ch.ring.nodes) == 0 {
		t.Fatal("Ring should have nodes")
	}

	// Test search with various hashes
	testCases := []struct {
		hash     uint32
		expected int // We can't predict exact index, but we can verify it's valid
	}{
		{0, 0},                    // Should wrap to first node
		{math.MaxUint32, 0},       // Should wrap to first node
		{ch.ring.nodes[0].hash, 0}, // Exact match
	}

	for _, tc := range testCases {
		idx := ch.search(tc.hash)
		if idx < 0 || idx >= len(ch.ring.nodes) {
			t.Errorf("Search for hash %d returned invalid index %d", tc.hash, idx)
		}
	}
}

// TestName tests the Name method.
func TestConsistentHashName(t *testing.T) {
	ch := NewConsistentHash(150)
	if ch.Name() != "consistent_hash" {
		t.Errorf("Expected name 'consistent_hash', got %q", ch.Name())
	}
}

// TestNextWithUnavailableBackend tests that Next skips unavailable backends.
func TestNextWithUnavailableBackend(t *testing.T) {
	ch := NewConsistentHash(150)

	// Create backends
	b1 := backend.NewBackend("backend1", "10.0.0.1:8080")
	b2 := backend.NewBackend("backend2", "10.0.0.2:8080")
	b3 := backend.NewBackend("backend3", "10.0.0.3:8080")

	// Set b2 as unavailable
	b2.SetState(backend.StateDown)

	ch.Add(b1)
	ch.Add(b2)
	ch.Add(b3)

	// Test Next with mixed available/unavailable backends
	backends := []*backend.Backend{b1, b2, b3}

	// Call Next multiple times and verify we never get the unavailable backend
	for i := 0; i < 100; i++ {
		result := ch.Next(backends)
		if result == nil {
			continue // May happen if selected backend is unavailable
		}
		if result.ID == "backend2" {
			t.Error("Next returned unavailable backend2")
		}
	}
}

// BenchmarkConsistentHash benchmarks the consistent hash algorithm.
func BenchmarkConsistentHash(b *testing.B) {
	ch := NewConsistentHash(150)

	// Add backends
	for i := 0; i < 10; i++ {
		be := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(be)
	}

	backends := make([]*backend.Backend, 10)
	for i := 0; i < 10; i++ {
		backends[i] = backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			hash := ch.hashKey(key)
			_ = ch.getNode(hash)
			i++
		}
	})
}

// BenchmarkConsistentHashNext benchmarks the Next method.
func BenchmarkConsistentHashNext(b *testing.B) {
	ch := NewConsistentHash(150)

	backends := make([]*backend.Backend, 10)
	for i := 0; i < 10; i++ {
		backends[i] = backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(backends[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Next(backends)
	}
}

// BenchmarkConsistentHashAdd benchmarks adding backends.
func BenchmarkConsistentHashAdd(b *testing.B) {
	ch := NewConsistentHash(150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		be := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(be)
		ch.Remove(be.ID) // Clean up to avoid unlimited growth
	}
}

// BenchmarkConsistentHashSearch benchmarks the binary search.
func BenchmarkConsistentHashSearch(b *testing.B) {
	ch := NewConsistentHash(150)

	// Add backends to create a large ring
	for i := 0; i < 100; i++ {
		be := backend.NewBackend(fmt.Sprintf("backend%d", i), fmt.Sprintf("10.0.0.%d:8080", i))
		ch.Add(be)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.search(uint32(i))
	}
}

// BenchmarkHashKey benchmarks the hash function.
func BenchmarkHashKey(b *testing.B) {
	ch := NewConsistentHash(150)
	keys := []string{"key1", "key2", "key3", "longer-key-for-testing", "user:123:profile"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.hashKey(keys[i%len(keys)])
	}
}

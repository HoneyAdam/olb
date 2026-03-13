package utils

import (
	"fmt"
	"testing"
)

func TestNewBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)

	if bf.Cap() == 0 {
		t.Error("Capacity should be > 0")
	}

	if bf.K() == 0 {
		t.Error("K should be > 0")
	}

	// Default values
	bf2 := NewBloomFilter(0, 0)
	if bf2.Cap() == 0 {
		t.Error("Should use default capacity when n=0")
	}
}

func TestBloomFilter_Basic(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)

	// Add some items
	items := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for _, item := range items {
		bf.AddString(item)
	}

	// Check all items are present
	for _, item := range items {
		if !bf.ContainsString(item) {
			t.Errorf("ContainsString(%s) = false, expected true", item)
		}
	}

	// Check non-existent items (may have false positives)
	notInFilter := []string{"grape", "kiwi", "mango"}
	falsePositives := 0
	for _, item := range notInFilter {
		if bf.ContainsString(item) {
			falsePositives++
		}
	}

	// Allow for some false positives with 1% rate
	if falsePositives > 1 {
		t.Logf("False positives: %d/%d", falsePositives, len(notInFilter))
	}
}

func TestBloomFilter_ByteSlice(t *testing.T) {
	bf := NewBloomFilter(100, 0.01)

	data := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("test"),
	}

	for _, d := range data {
		bf.Add(d)
	}

	for _, d := range data {
		if !bf.Contains(d) {
			t.Errorf("Contains(%s) = false", string(d))
		}
	}
}

func TestBloomFilter_Clear(t *testing.T) {
	bf := NewBloomFilter(100, 0.01)

	bf.AddString("test")
	if !bf.ContainsString("test") {
		t.Error("Item should be present before Clear")
	}

	bf.Clear()

	if bf.ContainsString("test") {
		t.Error("Item should not be present after Clear")
	}

	if bf.Len() != 0 {
		t.Errorf("Len() = %d, want 0", bf.Len())
	}
}

func TestBloomFilter_Len(t *testing.T) {
	bf := NewBloomFilter(100, 0.01)

	if bf.Len() != 0 {
		t.Errorf("Initial Len() = %d, want 0", bf.Len())
	}

	bf.AddString("a")
	if bf.Len() != 1 {
		t.Errorf("Len() = %d, want 1", bf.Len())
	}

	bf.AddString("b")
	if bf.Len() != 2 {
		t.Errorf("Len() = %d, want 2", bf.Len())
	}

	// Adding same item again
	bf.AddString("a")
	if bf.Len() != 3 {
		// Bloom filter doesn't track duplicates
		t.Errorf("Len() = %d, want 3", bf.Len())
	}
}

func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	// Test with high false positive rate
	bf := NewBloomFilter(100, 0.5) // 50% FP rate

	// Add items
	for i := 0; i < 50; i++ {
		bf.AddString(fmt.Sprintf("item%d", i))
	}

	// Check that items are present
	for i := 0; i < 50; i++ {
		if !bf.ContainsString(fmt.Sprintf("item%d", i)) {
			t.Errorf("Item %d should be present", i)
		}
	}

	// Check FP rate
	fpCount := 0
	for i := 100; i < 200; i++ {
		if bf.ContainsString(fmt.Sprintf("item%d", i)) {
			fpCount++
		}
	}

	t.Logf("False positive rate: %d%% (%d/100)", fpCount, fpCount)
}

func TestBloomFilter_Merge(t *testing.T) {
	bf1 := NewBloomFilter(100, 0.01)
	bf2 := NewBloomFilter(100, 0.01)

	bf1.AddString("a")
	bf2.AddString("b")

	err := bf1.Merge(bf2)
	if err != nil {
		t.Errorf("Merge failed: %v", err)
	}

	if !bf1.ContainsString("a") {
		t.Error("Should contain 'a' after merge")
	}
	if !bf1.ContainsString("b") {
		t.Error("Should contain 'b' after merge")
	}
}

func TestBloomFilter_MergeIncompatible(t *testing.T) {
	bf1 := NewBloomFilter(100, 0.01)
	bf2 := NewBloomFilter(200, 0.01)

	err := bf1.Merge(bf2)
	if err == nil {
		t.Error("Merge should fail with different capacities")
	}
}

func TestOptimalBloomFilter(t *testing.T) {
	m, k := OptimalBloomFilter(1000, 0.01)

	if m == 0 {
		t.Error("m should be > 0")
	}
	if k == 0 {
		t.Error("k should be > 0")
	}

	// Verify m is multiple of 64
	if m%64 != 0 {
		t.Errorf("m should be multiple of 64, got %d", m)
	}

	// Estimate false positive rate
	fpRate := EstimateFalsePositiveRate(1000, m, k)
	t.Logf("Estimated FP rate: %f", fpRate)
}

func BenchmarkBloomFilter_Add(b *testing.B) {
	bf := NewBloomFilter(10000, 0.01)
	data := []byte("benchmark data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(data)
	}
}

func BenchmarkBloomFilter_Contains(b *testing.B) {
	bf := NewBloomFilter(10000, 0.01)
	data := []byte("benchmark data")
	bf.Add(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(data)
	}
}

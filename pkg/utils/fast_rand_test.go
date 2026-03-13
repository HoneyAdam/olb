package utils

import (
	"sync"
	"testing"
)

func TestFastRand_Basic(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	// Uint64
	v1 := fr.Uint64()
	v2 := fr.Uint64()
	if v1 == v2 {
		t.Error("Consecutive Uint64 calls should return different values")
	}

	// Int63
	n1 := fr.Int63()
	n2 := fr.Int63()
	if n1 == n2 {
		t.Error("Consecutive Int63 calls should return different values")
	}
	if n1 < 0 {
		t.Error("Int63 should return non-negative value")
	}

	// Int63n
	for i := 0; i < 100; i++ {
		n := fr.Int63n(100)
		if n < 0 || n >= 100 {
			t.Errorf("Int63n(100) = %d, should be in [0, 100)", n)
		}
	}

	// Int63n with 0
	n := fr.Int63n(0)
	if n != 0 {
		t.Errorf("Int63n(0) = %d, want 0", n)
	}

	// Float64
	f1 := fr.Float64()
	f2 := fr.Float64()
	if f1 == f2 {
		t.Error("Consecutive Float64 calls should return different values")
	}
	if f1 < 0 || f1 >= 1 {
		t.Errorf("Float64 = %f, should be in [0, 1)", f1)
	}

	// Bool
	trueCount := 0
	for i := 0; i < 1000; i++ {
		if fr.Bool() {
			trueCount++
		}
	}
	// Should be roughly 50/50
	if trueCount < 400 || trueCount > 600 {
		t.Errorf("Bool distribution: %d true out of 1000, expected ~500", trueCount)
	}
}

func TestFastRand_Seed(t *testing.T) {
	// Same seed should produce same sequence
	fr1 := NewFastRandWithSeed(12345)
	fr2 := NewFastRandWithSeed(12345)

	for i := 0; i < 100; i++ {
		if fr1.Uint64() != fr2.Uint64() {
			t.Error("Same seed should produce same sequence")
			break
		}
	}

	// Different seeds should produce different sequences (with high probability)
	fr3 := NewFastRandWithSeed(12346)
	fr4 := NewFastRandWithSeed(12345)

	different := false
	for i := 0; i < 10; i++ {
		if fr3.Uint64() != fr4.Uint64() {
			different = true
			break
		}
	}
	if !different {
		t.Error("Different seeds should produce different sequences")
	}
}

func TestFastRand_Shuffle(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	items := []int{1, 2, 3, 4, 5}
	original := make([]int, len(items))
	copy(original, items)

	fr.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})

	// Check all elements are still present
	sum := 0
	for _, v := range items {
		sum += v
	}
	if sum != 15 {
		t.Error("Shuffle lost or duplicated elements")
	}

	// Check it's actually shuffled (may fail rarely due to chance)
	same := true
	for i := range items {
		if items[i] != original[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Shuffle didn't change order")
	}
}

func TestFastRand_Distribution(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	buckets := make([]int, 10)
	for i := 0; i < 10000; i++ {
		n := fr.Intn(10)
		buckets[n]++
	}

	// Check distribution is roughly uniform
	expected := 1000 // 10000 / 10
	for i, count := range buckets {
		if count < expected-200 || count > expected+200 {
			t.Errorf("Bucket %d has %d items, expected ~%d", i, count, expected)
		}
	}
}

func TestFastRand_Concurrent(t *testing.T) {
	// Each goroutine should have its own FastRand instance
	// This test verifies thread safety when used correctly

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			fr := NewFastRandWithSeed(uint64(seed))
			for j := 0; j < 1000; j++ {
				_ = fr.Uint64()
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkFastRand_Uint64(b *testing.B) {
	fr := NewFastRand()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fr.Uint64()
	}
}

func BenchmarkFastRand_Int63n(b *testing.B) {
	fr := NewFastRand()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fr.Int63n(100)
	}
}

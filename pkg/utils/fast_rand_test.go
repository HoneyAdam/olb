package utils

import (
	"fmt"
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

// TestFastRandDistribution tests distribution of random numbers
func TestFastRandDistribution(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	// Test Intn distribution
	t.Run("Intn distribution", func(t *testing.T) {
		buckets := make([]int, 10)
		for i := 0; i < 10000; i++ {
			n := fr.Intn(10)
			if n < 0 || n >= 10 {
				t.Errorf("Intn(10) = %d, out of range", n)
			}
			buckets[n]++
		}

		// Check distribution is roughly uniform (each bucket should have ~1000)
		expected := 1000
		for i, count := range buckets {
			if count < expected-300 || count > expected+300 {
				t.Logf("Bucket %d has %d items (expected ~%d)", i, count, expected)
			}
		}
	})

	// Test Uint64 distribution (check high and low bits)
	t.Run("Uint64 distribution", func(t *testing.T) {
		var hasHighBits, hasLowBits bool
		for i := 0; i < 1000; i++ {
			v := fr.Uint64()
			if v&0xFF00000000000000 != 0 {
				hasHighBits = true
			}
			if v&0xFF != 0 {
				hasLowBits = true
			}
		}
		if !hasHighBits {
			t.Error("No high bits set in 1000 iterations")
		}
		if !hasLowBits {
			t.Error("No low bits set in 1000 iterations")
		}
	})

	// Test Float64 distribution
	t.Run("Float64 distribution", func(t *testing.T) {
		buckets := make([]int, 10)
		for i := 0; i < 10000; i++ {
			f := fr.Float64()
			if f < 0 || f >= 1 {
				t.Errorf("Float64() = %f, out of range [0, 1)", f)
			}
			bucket := int(f * 10)
			if bucket == 10 {
				bucket = 9
			}
			buckets[bucket]++
		}

		expected := 1000
		for i, count := range buckets {
			if count < expected-300 || count > expected+300 {
				t.Logf("Float64 bucket %d has %d items (expected ~%d)", i, count, expected)
			}
		}
	})
}

// TestIntnBoundary tests Intn boundary conditions
func TestIntnBoundary(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	tests := []struct {
		n      int
		minVal int
		maxVal int
	}{
		{1, 0, 0},
		{2, 0, 1},
		{10, 0, 9},
		{100, 0, 99},
		{1000, 0, 999},
		{1000000, 0, 999999},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Intn(%d)", tt.n), func(t *testing.T) {
			minObserved := tt.n
			maxObserved := -1

			for i := 0; i < tt.n*100; i++ {
				val := fr.Intn(tt.n)
				if val < tt.minVal || val > tt.maxVal {
					t.Errorf("Intn(%d) = %d, out of range [%d, %d]",
						tt.n, val, tt.minVal, tt.maxVal)
				}
				if val < minObserved {
					minObserved = val
				}
				if val > maxObserved {
					maxObserved = val
				}
			}

			// For larger n, we should see both boundaries
			if tt.n >= 10 {
				if minObserved != 0 {
					t.Logf("Warning: min value %d not observed for Intn(%d)", 0, tt.n)
				}
				if maxObserved != tt.maxVal {
					t.Logf("Warning: max value %d not observed for Intn(%d)", tt.maxVal, tt.n)
				}
			}
		})
	}

	// Test edge cases
	t.Run("Intn(0)", func(t *testing.T) {
		val := fr.Intn(0)
		if val != 0 {
			t.Errorf("Intn(0) = %d, want 0", val)
		}
	})

	t.Run("Intn(-1)", func(t *testing.T) {
		val := fr.Intn(-1)
		if val != 0 {
			t.Errorf("Intn(-1) = %d, want 0", val)
		}
	})
}

// TestFloat64Range tests Float64 range
func TestFloat64Range(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	minVal := 1.0
	maxVal := 0.0

	for i := 0; i < 10000; i++ {
		f := fr.Float64()

		if f < 0 {
			t.Errorf("Float64() = %f, less than 0", f)
		}
		if f >= 1 {
			t.Errorf("Float64() = %f, greater or equal to 1", f)
		}

		if f < minVal {
			minVal = f
		}
		if f > maxVal {
			maxVal = f
		}
	}

	t.Logf("Float64 range observed: [%f, %f]", minVal, maxVal)

	// Should observe values close to 0 and close to 1
	if minVal > 0.0001 {
		t.Logf("Warning: minimum value %f not very close to 0", minVal)
	}
	if maxVal < 0.9999 {
		t.Logf("Warning: maximum value %f not very close to 1", maxVal)
	}
}

// TestFastRandAllMethods tests all methods of FastRand
func TestFastRandAllMethods(t *testing.T) {
	fr := NewFastRandWithSeed(12345)

	// Uint64
	t.Run("Uint64", func(t *testing.T) {
		v1 := fr.Uint64()
		v2 := fr.Uint64()
		if v1 == v2 {
			t.Error("Consecutive Uint64 calls should return different values")
		}
	})

	// Int63
	t.Run("Int63", func(t *testing.T) {
		v := fr.Int63()
		if v < 0 {
			t.Error("Int63 should return non-negative value")
		}
	})

	// Int63n
	t.Run("Int63n", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			v := fr.Int63n(100)
			if v < 0 || v >= 100 {
				t.Errorf("Int63n(100) = %d, out of range", v)
			}
		}
	})

	// Uint32
	t.Run("Uint32", func(t *testing.T) {
		v1 := fr.Uint32()
		v2 := fr.Uint32()
		if v1 == v2 {
			t.Error("Consecutive Uint32 calls should return different values")
		}
	})

	// Int31
	t.Run("Int31", func(t *testing.T) {
		v := fr.Int31()
		if v < 0 {
			t.Error("Int31 should return non-negative value")
		}
	})

	// Int31n
	t.Run("Int31n", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			v := fr.Int31n(100)
			if v < 0 || v >= 100 {
				t.Errorf("Int31n(100) = %d, out of range", v)
			}
		}
	})

	// Int
	t.Run("Int", func(t *testing.T) {
		v := fr.Int()
		if v < 0 {
			t.Error("Int should return non-negative value")
		}
	})

	// Intn
	t.Run("Intn", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			v := fr.Intn(100)
			if v < 0 || v >= 100 {
				t.Errorf("Intn(100) = %d, out of range", v)
			}
		}
	})

	// Float32
	t.Run("Float32", func(t *testing.T) {
		f := fr.Float32()
		if f < 0 || f >= 1 {
			t.Errorf("Float32() = %f, out of range [0, 1)", f)
		}
	})

	// Bool
	t.Run("Bool", func(t *testing.T) {
		trueCount := 0
		for i := 0; i < 1000; i++ {
			if fr.Bool() {
				trueCount++
			}
		}
		// Should be roughly 50/50
		if trueCount < 400 || trueCount > 600 {
			t.Errorf("Bool distribution: %d true out of 1000", trueCount)
		}
	})
}

// --- Tests for global/default FastRand functions ---

func TestRandUint64(t *testing.T) {
	v1 := RandUint64()
	v2 := RandUint64()
	// Two consecutive calls should almost certainly differ
	if v1 == v2 {
		t.Error("RandUint64: consecutive calls returned identical values")
	}
}

func TestRandInt63(t *testing.T) {
	v := RandInt63()
	if v < 0 {
		t.Errorf("RandInt63() = %d, expected non-negative", v)
	}
}

func TestRandInt63n(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := RandInt63n(50)
		if v < 0 || v >= 50 {
			t.Errorf("RandInt63n(50) = %d, out of range [0, 50)", v)
		}
	}
	// Edge case: n <= 0 returns 0
	if v := RandInt63n(0); v != 0 {
		t.Errorf("RandInt63n(0) = %d, want 0", v)
	}
	if v := RandInt63n(-1); v != 0 {
		t.Errorf("RandInt63n(-1) = %d, want 0", v)
	}
}

func TestRandInt(t *testing.T) {
	v := RandInt()
	if v < 0 {
		t.Errorf("RandInt() = %d, expected non-negative", v)
	}
}

func TestRandIntn(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := RandIntn(10)
		if v < 0 || v >= 10 {
			t.Errorf("RandIntn(10) = %d, out of range [0, 10)", v)
		}
	}
	// Edge case: n <= 0 returns 0
	if v := RandIntn(0); v != 0 {
		t.Errorf("RandIntn(0) = %d, want 0", v)
	}
	if v := RandIntn(-5); v != 0 {
		t.Errorf("RandIntn(-5) = %d, want 0", v)
	}
}

func TestRandFloat64(t *testing.T) {
	for i := 0; i < 100; i++ {
		f := RandFloat64()
		if f < 0 || f >= 1 {
			t.Errorf("RandFloat64() = %f, out of range [0, 1)", f)
		}
	}
}

func TestRandBool(t *testing.T) {
	trueCount := 0
	for i := 0; i < 1000; i++ {
		if RandBool() {
			trueCount++
		}
	}
	// Should be roughly 50/50
	if trueCount < 350 || trueCount > 650 {
		t.Errorf("RandBool distribution: %d true out of 1000, expected ~500", trueCount)
	}
}

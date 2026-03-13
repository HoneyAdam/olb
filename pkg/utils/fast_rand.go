package utils

import (
	"sync"
	"sync/atomic"
	"time"
)

// FastRand is a fast pseudo-random number generator using SplitMix64 algorithm.
// It's not cryptographically secure but is extremely fast and suitable for
// load balancing, shuffling, and other non-security purposes.
//
// Thread-safe for concurrent use.
type FastRand struct {
	state atomic.Uint64
}

// NewFastRand creates a new FastRand generator with a seed based on time.
func NewFastRand() *FastRand {
	return NewFastRandWithSeed(uint64(time.Now().UnixNano()))
}

// NewFastRandWithSeed creates a new FastRand generator with the given seed.
func NewFastRandWithSeed(seed uint64) *FastRand {
	fr := &FastRand{}
	fr.state.Store(seed)
	return fr
}

// Uint64 returns a random uint64.
func (fr *FastRand) Uint64() uint64 {
	// SplitMix64 algorithm
	x := fr.state.Add(0x9e3779b97f4a7c15)
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Int63 returns a random non-negative int63.
func (fr *FastRand) Int63() int64 {
	return int64(fr.Uint64() >> 1)
}

// Int63n returns a random int63 in [0, n).
func (fr *FastRand) Int63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return fr.Int63() % n
}

// Uint32 returns a random uint32.
func (fr *FastRand) Uint32() uint32 {
	return uint32(fr.Uint64())
}

// Int31 returns a random non-negative int31.
func (fr *FastRand) Int31() int32 {
	return int32(fr.Uint64() >> 33)
}

// Int31n returns a random int31 in [0, n).
func (fr *FastRand) Int31n(n int32) int32 {
	if n <= 0 {
		return 0
	}
	return fr.Int31() % n
}

// Int returns a random non-negative int.
func (fr *FastRand) Int() int {
	return int(fr.Int63())
}

// Intn returns a random int in [0, n).
func (fr *FastRand) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(fr.Int63n(int64(n)))
}

// Float64 returns a random float64 in [0.0, 1.0).
func (fr *FastRand) Float64() float64 {
	return float64(fr.Int63()) / (1 << 63)
}

// Float32 returns a random float32 in [0.0, 1.0).
func (fr *FastRand) Float32() float32 {
	return float32(fr.Float64())
}

// Bool returns a random boolean.
func (fr *FastRand) Bool() bool {
	return fr.Uint64()&1 == 0
}

// Shuffle randomly permutes n elements by calling swap(i, j).
func (fr *FastRand) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		j := fr.Intn(i + 1)
		swap(i, j)
	}
}

// defaultFastRand is the global FastRand instance.
var defaultFastRand = NewFastRand()
var defaultMu sync.Mutex

// Global functions using the default instance.

// RandUint64 returns a random uint64 from the default generator.
func RandUint64() uint64 { return defaultFastRand.Uint64() }

// RandInt63 returns a random int63 from the default generator.
func RandInt63() int64 { return defaultFastRand.Int63() }

// RandInt63n returns a random int63 in [0, n) from the default generator.
func RandInt63n(n int64) int64 { return defaultFastRand.Int63n(n) }

// RandInt returns a random int from the default generator.
func RandInt() int { return defaultFastRand.Int() }

// RandIntn returns a random int in [0, n) from the default generator.
func RandIntn(n int) int {
	if n <= 0 {
		return 0
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultFastRand.Intn(n)
}

// RandFloat64 returns a random float64 in [0.0, 1.0) from the default generator.
func RandFloat64() float64 { return defaultFastRand.Float64() }

// RandBool returns a random boolean from the default generator.
func RandBool() bool { return defaultFastRand.Bool() }

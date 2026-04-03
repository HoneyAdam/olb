package utils

import (
	"math"
	"sync/atomic"
	"time"
)

// AtomicFloat64 provides atomic operations on float64 values.
// Uses math.Float64bits/Float64frombits for atomic access.
type AtomicFloat64 struct {
	v atomic.Uint64
}

// NewAtomicFloat64 creates a new AtomicFloat64 with the given initial value.
func NewAtomicFloat64(val float64) *AtomicFloat64 {
	af := &AtomicFloat64{}
	af.Store(val)
	return af
}

// Load atomically loads and returns the float64 value.
func (af *AtomicFloat64) Load() float64 {
	return bitsToFloat64(af.v.Load())
}

// Store atomically stores the float64 value.
func (af *AtomicFloat64) Store(val float64) {
	af.v.Store(float64ToBits(val))
}

// Add atomically adds delta to the value and returns the new value.
func (af *AtomicFloat64) Add(delta float64) float64 {
	for {
		old := af.v.Load()
		newVal := bitsToFloat64(old) + delta
		if af.v.CompareAndSwap(old, float64ToBits(newVal)) {
			return newVal
		}
	}
}

// Sub atomically subtracts delta from the value and returns the new value.
func (af *AtomicFloat64) Sub(delta float64) float64 {
	return af.Add(-delta)
}

// CompareAndSwap executes the compare-and-swap operation for the float64 value.
func (af *AtomicFloat64) CompareAndSwap(old, new float64) bool {
	return af.v.CompareAndSwap(float64ToBits(old), float64ToBits(new))
}

// float64ToBits converts float64 to uint64 bits.
func float64ToBits(f float64) uint64 {
	return math.Float64bits(f)
}

// bitsToFloat64 converts uint64 bits to float64.
func bitsToFloat64(u uint64) float64 {
	return math.Float64frombits(u)
}

// AtomicDuration provides atomic operations on time.Duration values.
type AtomicDuration struct {
	v atomic.Int64
}

// NewAtomicDuration creates a new AtomicDuration with the given initial value.
func NewAtomicDuration(val time.Duration) *AtomicDuration {
	ad := &AtomicDuration{}
	ad.Store(val)
	return ad
}

// Load atomically loads and returns the duration value.
func (ad *AtomicDuration) Load() time.Duration {
	return time.Duration(ad.v.Load())
}

// Store atomically stores the duration value.
func (ad *AtomicDuration) Store(val time.Duration) {
	ad.v.Store(int64(val))
}

// Add atomically adds delta to the value and returns the new value.
func (ad *AtomicDuration) Add(delta time.Duration) time.Duration {
	return time.Duration(ad.v.Add(int64(delta)))
}

// Sub atomically subtracts delta from the value and returns the new value.
func (ad *AtomicDuration) Sub(delta time.Duration) time.Duration {
	return time.Duration(ad.v.Add(-int64(delta)))
}

// CompareAndSwap executes the compare-and-swap operation.
func (ad *AtomicDuration) CompareAndSwap(old, new time.Duration) bool {
	return ad.v.CompareAndSwap(int64(old), int64(new))
}

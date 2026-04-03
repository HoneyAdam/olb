// Package utils provides core utility types and functions used throughout OpenLoadBalancer.
// These are low-level, performance-critical primitives with zero external dependencies.
package utils

import (
	"sync"
)

// Size tiers for buffer pool.
// These are chosen based on common use cases:
//   - Small: headers, small JSON payloads
//   - Medium: typical HTTP requests/responses
//   - Large: large payloads, file transfers
//   - XLarge: very large payloads
const (
	SmallBufferSize  = 1 << 10  // 1 KB
	MediumBufferSize = 4 << 10  // 4 KB
	LargeBufferSize  = 16 << 10 // 16 KB
	XLargeBufferSize = 64 << 10 // 64 KB

	maxBufferSize = XLargeBufferSize
)

// BufferPool provides a tiered sync.Pool-based buffer management system.
// It reduces GC pressure by reusing byte slices across requests.
//
// The pool maintains separate sync.Pools for different size tiers.
// When Get is called, it returns a buffer from the appropriate tier
// based on the requested size. When Put is called, the buffer is
// returned to its original tier for reuse.
//
// Thread-safe for concurrent use.
type BufferPool struct {
	small  *sync.Pool // 1 KB buffers
	medium *sync.Pool // 4 KB buffers
	large  *sync.Pool // 16 KB buffers
	xlarge *sync.Pool // 64 KB buffers
}

// NewBufferPool creates a new BufferPool with initialized pools for each tier.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		small: &sync.Pool{
			New: func() any {
				b := make([]byte, SmallBufferSize)
				return &b
			},
		},
		medium: &sync.Pool{
			New: func() any {
				b := make([]byte, MediumBufferSize)
				return &b
			},
		},
		large: &sync.Pool{
			New: func() any {
				b := make([]byte, LargeBufferSize)
				return &b
			},
		},
		xlarge: &sync.Pool{
			New: func() any {
				b := make([]byte, XLargeBufferSize)
				return &b
			},
		},
	}
}

// DefaultBufferPool is the global buffer pool instance.
// Use this for most cases rather than creating custom pools.
var DefaultBufferPool = NewBufferPool()

// Get retrieves a buffer from the pool with at least the requested size.
// The returned buffer may be larger than requested but never smaller.
// The buffer is zeroed before being returned.
//
// Callers must call Put when done with the buffer to return it to the pool.
//
// For sizes larger than maxBufferSize (64 KB), a new buffer is allocated
// and not pooled (it's passed to GC when discarded).
func (p *BufferPool) Get(size int) []byte {
	if size <= 0 {
		return nil
	}

	// Oversized allocation - not pooled
	if size > maxBufferSize {
		return make([]byte, size)
	}

	// Select appropriate pool based on size
	var pool *sync.Pool
	switch {
	case size <= SmallBufferSize:
		pool = p.small
	case size <= MediumBufferSize:
		pool = p.medium
	case size <= LargeBufferSize:
		pool = p.large
	default:
		pool = p.xlarge
	}

	// Get from pool
	v := pool.Get()
	if v == nil {
		// Should not happen due to Pool.New, but handle defensively
		return make([]byte, size)
	}

	buf := *(v.(*[]byte))

	// Ensure buffer is large enough (should always be true for pooled buffers)
	if len(buf) < size {
		// Return to pool and allocate new (should not happen)
		pool.Put(v)
		return make([]byte, size)
	}

	// Zero the buffer for security (prevent data leakage)
	// Only zero what we need
	clear(buf[:size])

	return buf[:size]
}

// Put returns a buffer to the pool for reuse.
// The buffer must have been obtained from Get on the same BufferPool.
//
// Buffers larger than maxBufferSize are not pooled and are left for GC.
// Nil or empty buffers are ignored.
func (p *BufferPool) Put(buf []byte) {
	if len(buf) == 0 {
		return
	}

	capacity := cap(buf)

	// Oversized buffers are not returned to pool
	if capacity > maxBufferSize {
		return
	}

	// Reset slice to full capacity before returning to pool
	buf = buf[:capacity]

	// Select appropriate pool based on capacity
	switch capacity {
	case SmallBufferSize:
		p.small.Put(&buf)
	case MediumBufferSize:
		p.medium.Put(&buf)
	case LargeBufferSize:
		p.large.Put(&buf)
	case XLargeBufferSize:
		p.xlarge.Put(&buf)
	default:
		// Unknown size - don't pool
		// This could happen if someone Put's a non-pool buffer
	}
}

// Get is a convenience wrapper around DefaultBufferPool.Get.
func Get(size int) []byte {
	return DefaultBufferPool.Get(size)
}

// Put is a convenience wrapper around DefaultBufferPool.Put.
func Put(buf []byte) {
	DefaultBufferPool.Put(buf)
}

// Copy creates a new independent copy of the data.
// The returned slice is obtained from the pool and should be Put when done.
func (p *BufferPool) Copy(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	buf := p.Get(len(data))
	copy(buf, data)
	return buf
}

// Copy is a convenience wrapper around DefaultBufferPool.Copy.
func Copy(data []byte) []byte {
	return DefaultBufferPool.Copy(data)
}

// Grow returns a buffer with at least the requested capacity.
// If the provided buffer has sufficient capacity, it is resized and returned.
// Otherwise, a new buffer is obtained from the pool and data is copied.
// The original buffer is returned to the pool if a new one is allocated.
func (p *BufferPool) Grow(buf []byte, needed int) []byte {
	if cap(buf) >= needed {
		return buf[:needed]
	}

	// Need larger buffer
	newBuf := p.Get(needed)
	copy(newBuf, buf)
	p.Put(buf)
	return newBuf
}

// Grow is a convenience wrapper around DefaultBufferPool.Grow.
func Grow(buf []byte, needed int) []byte {
	return DefaultBufferPool.Grow(buf, needed)
}

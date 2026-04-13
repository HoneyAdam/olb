package metrics

import (
	"runtime"
	"sync/atomic"
)

// numShards is the number of counter shards. Must be a power of two.
var numShards = func() int {
	n := runtime.NumCPU()
	// Round up to next power of two
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n++
	if n < 8 {
		n = 8
	}
	return n
}()

// shardMask is numShards - 1, used for fast modulo via bitwise AND.
var shardMask = numShards - 1

// goroutineID counter for cheap goroutine-local sharding.
var gidCounter atomic.Int64

// Counter is a sharded atomic int64 counter optimized for high-concurrency writes.
// It distributes increments across multiple shards to reduce cache-line contention.
type Counter struct {
	shards []atomic.Int64
	name   string
	help   string
}

// NewCounter creates a new sharded counter.
func NewCounter(name, help string) *Counter {
	return &Counter{
		shards: make([]atomic.Int64, numShards),
		name:   name,
		help:   help,
	}
}

// Name returns the counter name.
func (c *Counter) Name() string {
	return c.name
}

// Help returns the counter help text.
func (c *Counter) Help() string {
	return c.help
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.Add(1)
}

// Add adds n to the counter.
func (c *Counter) Add(n int64) {
	shard := int(uint64(gidCounter.Add(1)) & uint64(shardMask))
	c.shards[shard].Add(n)
}

// Get returns the current counter value by summing all shards.
func (c *Counter) Get() int64 {
	var total int64
	for i := range c.shards {
		total += c.shards[i].Load()
	}
	return total
}

// Reset resets all shards to 0.
func (c *Counter) Reset() {
	for i := range c.shards {
		c.shards[i].Store(0)
	}
}

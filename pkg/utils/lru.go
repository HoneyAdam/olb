package utils

import (
	"container/list"
	"sync"
	"time"
)

// entry is an internal wrapper for cache entries.
type entry[K comparable, V any] struct {
	key        K
	value      V
	expiration int64 // Unix nano, 0 means no expiration
}

// isExpired returns true if the entry has expired.
func (e *entry[K, V]) isExpired() bool {
	if e.expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > e.expiration
}

// LRU is a thread-safe LRU cache with optional TTL support.
//
// Type parameters:
//   - K: the key type, must be comparable
//   - V: the value type
//
// The cache has a fixed capacity. When the cache is full and a new item
// is added, the least recently used item is evicted.
//
// Thread-safe for concurrent use.
type LRU[K comparable, V any] struct {
	capacity int
	items    map[K]*list.Element // key -> list element
	order    *list.List          // LRU order: front = most recent
	mu       sync.RWMutex
	onEvict  func(key K, value V) // optional eviction callback
}

// NewLRU creates a new LRU cache with the given capacity.
// Returns an error if capacity is <= 0.
func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	return &LRU[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}, nil
}

// MustNewLRU creates a new LRU cache, panicking on error.
func MustNewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	lru, err := NewLRU[K, V](capacity)
	if err != nil {
		panic(err)
	}
	return lru
}

// SetOnEvict sets a callback function called when entries are evicted.
// The callback is called with the evicted key and value.
func (c *LRU[K, V]) SetOnEvict(fn func(key K, value V)) {
	c.mu.Lock()
	c.onEvict = fn
	c.mu.Unlock()
}

// Capacity returns the maximum number of items the cache can hold.
func (c *LRU[K, V]) Capacity() int {
	return c.capacity
}

// Len returns the current number of items in the cache.
func (c *LRU[K, V]) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired.
// Returns zero value and false if not found or expired.
// Expired entries are automatically removed.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	ent := elem.Value.(*entry[K, V])

	// Check if expired
	if ent.isExpired() {
		c.removeElement(elem)
		var zero V
		return zero, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)

	return ent.value, true
}

// Put adds or updates a value in the cache.
// If the key already exists, the value is updated and moved to front.
// If the cache is full, the least recently used item is evicted.
func (c *LRU[K, V]) Put(key K, value V) {
	c.PutWithTTL(key, value, 0)
}

// PutWithTTL adds or updates a value with a TTL (time-to-live).
// After the duration expires, the entry will be treated as missing.
// A TTL of 0 means no expiration.
func (c *LRU[K, V]) PutWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing
		ent := elem.Value.(*entry[K, V])
		ent.value = value
		ent.expiration = expiration
		c.order.MoveToFront(elem)
		return
	}

	// Add new entry
	ent := &entry[K, V]{
		key:        key,
		value:      value,
		expiration: expiration,
	}
	elem := c.order.PushFront(ent)
	c.items[key] = elem

	// Evict oldest if over capacity
	if len(c.items) > c.capacity {
		c.evictOldest()
	}
}

// Contains checks if a key exists in the cache without updating access order.
// Returns false for expired entries and removes them.
func (c *LRU[K, V]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	ent := elem.Value.(*entry[K, V])
	if ent.isExpired() {
		c.removeElement(elem)
		return false
	}

	return true
}

// Peek retrieves a value without updating the access order.
// Returns the value and true if found and not expired.
// Expired entries are evicted (consistent with Get/Contains).
func (c *LRU[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	ent := elem.Value.(*entry[K, V])
	if ent.isExpired() {
		c.removeElement(elem)
		var zero V
		return zero, false
	}

	return ent.value, true
}

// Delete removes a key from the cache.
// Returns true if the key was present.
func (c *LRU[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeElement(elem)
	return true
}

// Clear removes all items from the cache.
func (c *LRU[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Call eviction callback for all items
	if c.onEvict != nil {
		for elem := c.order.Back(); elem != nil; elem = elem.Prev() {
			ent := elem.Value.(*entry[K, V])
			c.onEvict(ent.key, ent.value)
		}
	}

	c.items = make(map[K]*list.Element, c.capacity)
	c.order.Init()
}

// Keys returns a slice of all keys in the cache.
// Expired keys are automatically removed.
// The order is from most recently used to least recently used.
func (c *LRU[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]K, 0, len(c.items))
	for elem := c.order.Front(); elem != nil; {
		next := elem.Next()
		ent := elem.Value.(*entry[K, V])

		if ent.isExpired() {
			c.removeElement(elem)
		} else {
			keys = append(keys, ent.key)
		}
		elem = next
	}

	return keys
}

// Values returns a slice of all values in the cache.
// Expired entries are automatically removed.
// The order is from most recently used to least recently used.
func (c *LRU[K, V]) Values() []V {
	c.mu.Lock()
	defer c.mu.Unlock()

	values := make([]V, 0, len(c.items))
	for elem := c.order.Front(); elem != nil; {
		next := elem.Next()
		ent := elem.Value.(*entry[K, V])

		if ent.isExpired() {
			c.removeElement(elem)
		} else {
			values = append(values, ent.value)
		}
		elem = next
	}

	return values
}

// Resize changes the capacity of the cache.
// If the new capacity is smaller, items are evicted to fit.
// Returns the number of items evicted.
func (c *LRU[K, V]) Resize(capacity int) int {
	if capacity <= 0 {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	evicted := 0
	for len(c.items) > capacity {
		c.evictOldest()
		evicted++
	}

	c.capacity = capacity
	return evicted
}

// evictOldest removes the least recently used item.
// Caller must hold lock.
func (c *LRU[K, V]) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
}

// removeElement removes an element from the cache.
// Caller must hold lock.
func (c *LRU[K, V]) removeElement(elem *list.Element) {
	ent := elem.Value.(*entry[K, V])
	delete(c.items, ent.key)
	c.order.Remove(elem)

	if c.onEvict != nil {
		c.onEvict(ent.key, ent.value)
	}
}

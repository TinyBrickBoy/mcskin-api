// Package cache provides a tiny, concurrency-safe TTL cache.
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// TTL is a generic in-memory cache where every entry expires after a fixed
// duration. It is safe for concurrent use.
type TTL[K comparable, V any] struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[K]entry[V]
}

// New returns a cache whose entries live for ttl.
func New[K comparable, V any](ttl time.Duration) *TTL[K, V] {
	return &TTL[K, V]{ttl: ttl, m: make(map[K]entry[V])}
}

// Get returns the cached value and true if present and not expired.
func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value under key with the cache's TTL.
func (c *TTL[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.m[key] = entry[V]{value: value, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

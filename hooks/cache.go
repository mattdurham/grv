package hooks

import (
	"os"
	"sync"
	"time"
)

type cacheKey struct {
	hookName    string
	absFilePath string
}

type cacheEntry struct {
	result map[string]interface{}
	mtime  time.Time
}

// Cache is a thread-safe hook result cache keyed by (hookName, absFilePath).
// Entries are invalidated when the file's mtime advances past the stored mtime.
type Cache struct {
	mu      sync.RWMutex
	entries map[cacheKey]cacheEntry
}

// NewCache creates an empty Cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[cacheKey]cacheEntry)}
}

// Get returns the cached result for (hookName, absFilePath) if it exists and
// the file has not been modified since the entry was stored.
func (c *Cache) Get(hookName, absFilePath string) (map[string]interface{}, bool) {
	k := cacheKey{hookName, absFilePath}

	c.mu.RLock()
	entry, ok := c.entries[k]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	fi, err := os.Stat(absFilePath)
	if err != nil {
		// File gone — evict
		c.mu.Lock()
		delete(c.entries, k)
		c.mu.Unlock()
		return nil, false
	}

	if fi.ModTime().After(entry.mtime) {
		// File modified — evict
		c.mu.Lock()
		delete(c.entries, k)
		c.mu.Unlock()
		return nil, false
	}

	return entry.result, true
}

// Set stores the result for (hookName, absFilePath) with the given file mtime.
func (c *Cache) Set(hookName, absFilePath string, result map[string]interface{}, mtime time.Time) {
	k := cacheKey{hookName, absFilePath}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[k] = cacheEntry{result, mtime}
}

// Invalidate removes all cached entries for absFilePath regardless of hook name.
func (c *Cache) Invalidate(absFilePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.absFilePath == absFilePath {
			delete(c.entries, k)
		}
	}
}

package history

import (
	"container/list"
	"sync"
)

type contentKey struct {
	kind                        byte
	hash, parent, before, after string
}
type cacheEntry struct {
	key   contentKey
	value any
	bytes int
}

// Each service belongs to one repository. Both content kinds share a bounded
// LRU; the mutex permits simultaneous history, preview and full-screen requests.
type contentCache struct {
	mu                          sync.Mutex
	entries                     map[contentKey]*list.Element
	order                       *list.List
	maxEntries, maxBytes, bytes int
}

func newContentCache(entries, bytes int) *contentCache {
	return &contentCache{entries: make(map[contentKey]*list.Element), order: list.New(), maxEntries: entries, maxBytes: bytes}
}
func (c *contentCache) get(k contentKey) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[k]; e != nil {
		c.order.MoveToFront(e)
		return e.Value.(cacheEntry).value, true
	}
	return nil, false
}
func (c *contentCache) put(k contentKey, value any, size int) {
	if c == nil {
		return
	}
	size += len(k.hash) + len(k.parent) + len(k.before) + len(k.after)
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[k]; e != nil {
		c.remove(e)
	}
	if size > c.maxBytes {
		return
	}
	c.entries[k] = c.order.PushFront(cacheEntry{k, value, size})
	c.bytes += size
	for c.order.Len() > c.maxEntries || c.bytes > c.maxBytes {
		c.remove(c.order.Back())
	}
}
func (c *contentCache) remove(e *list.Element) {
	entry := e.Value.(cacheEntry)
	delete(c.entries, entry.key)
	c.bytes -= entry.bytes
	c.order.Remove(e)
}

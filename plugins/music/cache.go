package music

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// ttlCache 是进程内带过期与容量上限的字节缓存。
//
// 音乐元数据只读，TTL 内直接复用序列化结果；容量上限避免搜索词无限增长。
type ttlCache struct {
	mu    sync.Mutex
	items map[string]cacheEntry
	max   int
}

func newTTLCache(max int) *ttlCache {
	return &ttlCache{items: make(map[string]cacheEntry), max: max}
}

func (c *ttlCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(item.expiresAt) {
		delete(c.items, key)
		return nil, false
	}
	return item.value, true
}

func (c *ttlCache) set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if len(c.items) >= c.max {
		// 先清过期项；仍满则淘汰任意项，容量上限优先于命中率
		for k, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, k)
			}
		}
		for k := range c.items {
			if len(c.items) < c.max {
				break
			}
			delete(c.items, k)
		}
	}
	c.items[key] = cacheEntry{value: value, expiresAt: now.Add(ttl)}
}

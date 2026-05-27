package storex

import (
	"context"
	"sync"
	"time"
)

// memValue 内存存储值结构
type memValue struct {
	value     string
	expiresAt int64 // Unix 时间戳（毫秒），0 表示永不过期
}

// MemStore 内存存储实现（用于测试/单机场景）
type MemStore struct {
	data   map[string]memValue
	mu     sync.RWMutex
	prefix string
}

// NewMemStore 创建内存存储实例
func NewMemStore(opts ...StoreOption) *MemStore {
	options := NewStoreOptions(opts...)
	return &MemStore{
		data:   make(map[string]memValue),
		prefix: options.Prefix,
	}
}

// key 添加前缀
func (m *MemStore) key(k string) string {
	if m.prefix == "" {
		return k
	}
	return m.prefix + ":" + k
}

// Set 设置键值对
func (m *MemStore) Set(ctx context.Context, key, value string, expire time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt int64
	if expire > 0 {
		expiresAt = time.Now().Add(expire).UnixMilli()
	}

	m.data[m.key(key)] = memValue{
		value:     value,
		expiresAt: expiresAt,
	}
	return nil
}

// Get 获取键值
func (m *MemStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.data[m.key(key)]
	if !ok {
		return "", nil
	}

	// 检查是否过期
	if v.expiresAt > 0 && time.Now().UnixMilli() > v.expiresAt {
		// 异步删除过期数据
		go func() {
			m.mu.Lock()
			delete(m.data, m.key(key))
			m.mu.Unlock()
		}()
		return "", nil
	}

	return v.value, nil
}

// Delete 删除键
func (m *MemStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, m.key(key))
	return nil
}

// Exists 检查键是否存在
func (m *MemStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.data[m.key(key)]
	if !ok {
		return false, nil
	}

	// 检查是否过期
	if v.expiresAt > 0 && time.Now().UnixMilli() > v.expiresAt {
		return false, nil
	}

	return true, nil
}

// Expire 设置过期时间
func (m *MemStore) Expire(ctx context.Context, key string, expire time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(key)
	v, ok := m.data[k]
	if !ok {
		return nil
	}

	if expire > 0 {
		v.expiresAt = time.Now().Add(expire).UnixMilli()
	} else {
		v.expiresAt = 0
	}
	m.data[k] = v
	return nil
}

// Clear 清空所有数据
func (m *MemStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]memValue)
	return nil
}

// Size 返回存储的键值对数量
func (m *MemStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
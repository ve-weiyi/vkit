package storex

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis 存储实现（基于 go-redis/v9）
type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore 创建 Redis 存储实例
func NewRedisStore(client *redis.Client, opts ...StoreOption) *RedisStore {
	options := NewStoreOptions(opts...)
	return &RedisStore{
		client: client,
		prefix: options.Prefix,
	}
}

// key 添加前缀
func (s *RedisStore) key(k string) string {
	if s.prefix == "" {
		return k
	}
	return s.prefix + ":" + k
}

// Set 设置键值对，expire=0 表示永不过期
func (s *RedisStore) Set(ctx context.Context, key, value string, expire time.Duration) error {
	return s.client.Set(ctx, s.key(key), value, expire).Err()
}

// Get 获取键值，key 不存在返回空字符串和 nil
func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, s.key(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Delete 删除键
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

// Exists 检查键是否存在
func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, s.key(key)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Expire 设置过期时间
func (s *RedisStore) Expire(ctx context.Context, key string, expire time.Duration) error {
	return s.client.Expire(ctx, s.key(key), expire).Err()
}

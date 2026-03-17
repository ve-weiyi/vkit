package tokenstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// 定义全局上下文，也可以让调用方传入
var ctx = context.Background()

// RedisStore Redis 存储实现（基于 go-redis/v8）
type RedisStore struct {
	client *redis.Client // 替换为 go-redis 的 Client
	prefix string
}

// NewRedisStore 创建 Redis 存储实例
func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{
		client: client,
		prefix: prefix,
	}
}

// key 添加前缀
func (s *RedisStore) key(k string) string {
	if s.prefix == "" {
		return k
	}
	return s.prefix + k
}

// Set 存储键值对并设置过期时间（秒）
func (s *RedisStore) Set(key string, value string, expireSeconds int) error {
	// go-redis 使用 time.Duration 作为过期时间单位，需要转换
	expire := time.Duration(expireSeconds) * time.Second
	return s.client.Set(ctx, s.key(key), value, expire).Err()
}

// Get 获取指定键的值
func (s *RedisStore) Get(key string) (string, error) {
	val, err := s.client.Get(ctx, s.key(key)).Result()
	// 适配原逻辑：key不存在时返回空字符串和nil，而非redis.Nil错误
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Delete 删除指定键
func (s *RedisStore) Delete(key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

// Exists 判断键是否存在
func (s *RedisStore) Exists(key string) (bool, error) {
	// go-redis 的 Exists 返回 int64 类型（存在返回1，不存在返回0）
	count, err := s.client.Exists(ctx, s.key(key)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetExpire 为指定键设置过期时间（秒）
func (s *RedisStore) SetExpire(key string, expireSeconds int) error {
	expire := time.Duration(expireSeconds) * time.Second
	return s.client.Expire(ctx, s.key(key), expire).Err()
}

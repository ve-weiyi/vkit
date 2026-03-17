package captcha

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore 验证码存储
type RedisStore struct {
	Redis      *redis.Client   // Redis 客户端
	Expiration time.Duration   // 过期时间，默认 15 分钟
	Context    context.Context // 上下文
}

// NewRedisStore 创建 Redis 存储实例
func NewRedisStore(rd *redis.Client) *RedisStore {
	return &RedisStore{
		Expiration: 15 * 60 * time.Second,
		Redis:      rd,
		Context:    context.Background(),
	}
}

func (rs *RedisStore) Set(key string, value string) error {
	err := rs.Redis.Set(rs.Context, key, value, rs.Expiration).Err()
	if err != nil {
		return err
	}

	return nil
}

func (rs *RedisStore) Get(key string, clear bool) string {
	val, err := rs.Redis.Get(rs.Context, key).Result()
	if err != nil {
		return ""
	}
	if clear {
		err := rs.Redis.Del(rs.Context, key).Err()
		if err != nil {
			return ""
		}
	}
	return val
}

// Verify 验证验证码
func (rs *RedisStore) Verify(key, answer string, clear bool) bool {
	v := rs.Get(key, clear)
	if v == "" {
		return false
	}
	return v == answer
}

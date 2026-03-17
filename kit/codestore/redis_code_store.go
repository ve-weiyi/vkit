package codestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/pkg/utils/randomx"
)

// RedisCodeStore 基于Redis的CodeStore实现
type RedisCodeStore struct {
	client *redis.Client
}

// NewRedisCodeStore 创建RedisCodeStore实例
func NewRedisCodeStore(client *redis.Client) *RedisCodeStore {
	return &RedisCodeStore{
		client: client,
	}
}

// Generate 生成并存储验证码（自动覆盖旧值）
func (r *RedisCodeStore) Generate(key string, length int, expire time.Duration) (string, error) {
	if key == "" {
		return "", errors.New("key is empty")
	}
	if length <= 0 {
		return "", errors.New("code length must > 0")
	}
	if expire <= 0 {
		expire = 5 * time.Minute
	}

	code := randomx.GenerateCode(length)
	err := r.client.Set(context.Background(), key, code, expire).Err()
	if err != nil {
		return "", fmt.Errorf("store code failed: %v", err)
	}

	return code, nil
}

// Verify 验证验证码（自动判断过期）
func (r *RedisCodeStore) Verify(key string, code string) (bool, error) {
	if key == "" || code == "" {
		return false, errors.New("key or code is empty")
	}

	storedCode, err := r.client.Get(context.Background(), key).Result()
	switch {
	case err == redis.Nil:
		return false, nil
	case err != nil:
		return false, fmt.Errorf("get code failed: %v", err)
	case storedCode != code:
		return false, nil
	default:
		_ = r.client.Del(context.Background(), key).Err()
		return true, nil
	}
}

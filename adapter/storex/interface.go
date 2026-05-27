package storex

import (
	"context"
	"time"
)

// KVStore 键值存储通用接口
// 所有存储实现（Redis/内存）都需实现此接口
type KVStore interface {
	// Set 设置键值对，expire=0 表示永不过期
	Set(ctx context.Context, key, value string, expire time.Duration) error

	// Get 获取键值，key 不存在返回空字符串和 nil
	Get(ctx context.Context, key string) (string, error)

	// Delete 删除键
	Delete(ctx context.Context, key string) error

	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// Expire 设置过期时间
	Expire(ctx context.Context, key string, expire time.Duration) error
}

// StoreOption 存储选项配置
type StoreOption func(*StoreOptions)

type StoreOptions struct {
	Prefix string
}

func WithPrefix(prefix string) StoreOption {
	return func(o *StoreOptions) {
		o.Prefix = prefix
	}
}

// NewStoreOptions 构建配置选项
func NewStoreOptions(opts ...StoreOption) *StoreOptions {
	options := &StoreOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
package redisstreamx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// RedisStream Redis Stream实现
type RedisStream struct {
	client *redis.Client
	config *Config
}

// Config Redis Stream配置
type Config struct {
	Addr     string // Redis地址
	Password string // Redis密码
	DB       int    // Redis数据库
}

// NewRedisStream 创建Redis Stream实例
func NewRedisStream(config *Config) (*RedisStream, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStream{
		client: client,
		config: config,
	}, nil
}

// Producer 实现 mqx.MessageQueue 接口
func (r *RedisStream) Producer() (mqx.Producer, error) {
	return newProducer(r.client, r.config)
}

// Consumer 实现 mqx.MessageQueue 接口
func (r *RedisStream) Consumer(config *mqx.ConsumerConfig) (mqx.Consumer, error) {
	return newConsumer(r.client, r.config, config)
}

// Close 实现 mqx.MessageQueue 接口
func (r *RedisStream) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Ping 实现 mqx.MessageQueue 接口
func (r *RedisStream) Ping(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("client is nil")
	}
	return r.client.Ping(ctx).Err()
}

// 确保实现了接口
var _ mqx.MessageQueue = (*RedisStream)(nil)

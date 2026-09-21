package redisstreamx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// conn Redis 连接封装。go-redis 内部管理连接池和自动重连，
// conn 仅作为共享 client 的持有者，遵循与 rabbitmqx 一致的接口风格。
type conn struct {
	client redis.UniversalClient
}

// dialConn 建立 Redis 连接并验证连通性。
func dialConn(cfg *Config) (*conn, error) {
	cfg.normalize()

	opts := &redis.UniversalOptions{
		Addrs:        cfg.Addrs,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	client := redis.NewUniversalClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redisstreamx: dial: %w", err)
	}

	return &conn{client: client}, nil
}

// Client 返回底层 redis.UniversalClient。
func (c *conn) Client() redis.UniversalClient {
	return c.client
}

// Ping 检测 Redis 连通性。
func (c *conn) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrNotConnected, err)
	}
	return nil
}

// Close 关闭所有连接。
func (c *conn) Close() error {
	return c.client.Close()
}

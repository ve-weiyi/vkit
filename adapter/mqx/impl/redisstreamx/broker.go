package redisstreamx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Broker 是 Redis Stream 的连接与订阅管理器。
type Broker struct {
	cfg    Config
	client redis.UniversalClient

	mu     sync.Mutex
	closed bool
	subs   map[*subscription]struct{}
}

var _ mqx.Broker = (*Broker)(nil)

// NewBroker 创建 Broker；LazyConnect 为 false 时立即验证连接。
func NewBroker(cfg *Config) (*Broker, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.normalize()

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        cfg.Addrs,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	if !cfg.LazyConnect {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("redisstreamx: dial: %w", err)
		}
	}

	return &Broker{cfg: *cfg, client: client, subs: make(map[*subscription]struct{})}, nil
}

// Publisher 实现 mqx.Broker。
func (b *Broker) Publisher() (mqx.Publisher, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, mqx.ErrClosed
	}
	return &publisher{broker: b}, nil
}

// Subscriber 实现 mqx.Broker。
func (b *Broker) Subscriber() (mqx.Subscriber, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, mqx.ErrClosed
	}
	return &subscriber{broker: b}, nil
}

// Ping 实现 mqx.Broker。
func (b *Broker) Ping(ctx context.Context) error {
	if err := b.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
	}
	return nil
}

// Close 先停止全部订阅，再关闭连接。
func (b *Broker) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	subs := make([]*subscription, 0, len(b.subs))
	for sub := range b.subs {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	var firstErr error
	for _, sub := range subs {
		if err := sub.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := b.client.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (b *Broker) register(sub *subscription) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return mqx.ErrClosed
	}
	b.subs[sub] = struct{}{}
	return nil
}

func (b *Broker) unregister(sub *subscription) {
	b.mu.Lock()
	delete(b.subs, sub)
	b.mu.Unlock()
}

func (b *Broker) logf(msg string, args ...any) {
	b.cfg.Logger.Error(msg, args...)
}

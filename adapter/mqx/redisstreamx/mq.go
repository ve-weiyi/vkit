package redisstreamx

import (
	"context"
	"fmt"
	"sync"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.MessageQueue = (*RedisStream)(nil)

// RedisStream Redis Stream 的 mqx.MessageQueue 实现。
// 通过 New() 创建，持有 Redis 连接并从中派生 Publisher / Subscriber。
type RedisStream struct {
	mu     sync.Mutex
	conn   *conn
	logger mqx.Logger
	closed bool
}

// New 创建 Redis Stream RedisStream。建立连接并验证连通性。
func New(cfg *Config) (*RedisStream, error) {
	cfg.normalize()

	conn, err := dialConn(cfg)
	if err != nil {
		return nil, err
	}

	return &RedisStream{conn: conn, logger: cfg.Logger}, nil
}

// Publisher 实现 mqx.MessageQueue。从共享连接派生 Publisher。
func (mq *RedisStream) Publisher(opts ...mqx.PublishOption) (mqx.Publisher, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	pub, err := newPublisher(mq.conn.Client(), opts)
	if err != nil {
		return nil, fmt.Errorf("redisstreamx: create publisher: %w", err)
	}
	return pub, nil
}

// Subscriber 实现 mqx.MessageQueue。从共享连接派生 Subscriber。
func (mq *RedisStream) Subscriber(opts ...mqx.SubscribeOption) (mqx.Subscriber, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	return newSubscriber(mq.conn.Client(), mq.logger, opts)
}

// Ping 实现 mqx.MessageQueue。检测 Redis 连通性。
func (mq *RedisStream) Ping(ctx context.Context) error {
	return mq.conn.Ping(ctx)
}

// Close 实现 mqx.MessageQueue。关闭 Redis 连接。
func (mq *RedisStream) Close() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil
	}
	mq.closed = true
	return mq.conn.Close()
}

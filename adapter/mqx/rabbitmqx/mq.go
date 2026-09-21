package rabbitmqx

import (
	"context"
	"fmt"
	"sync"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.MessageQueue = (*RabbitMQ)(nil)

// RabbitMQ RabbitMQ 的 mqx.MessageQueue 实现。
// 通过 New() 创建，持有连接并从中派生 Publisher / Subscriber。
type RabbitMQ struct {
	mu       sync.Mutex
	conn     *rabbitMQConn
	topology *topology
	logger   mqx.Logger
	closed   bool
}

// New 创建 RabbitMQ RabbitMQ。建立连接并声明 Exchange 拓扑。
func New(cfg *Config) (*RabbitMQ, error) {
	cfg.normalize()

	conn, err := dialConn(cfg)
	if err != nil {
		return nil, err
	}

	return &RabbitMQ{
		conn:     conn,
		topology: newTopology(cfg.ExchangeName, cfg.ExchangeType, cfg.Durable, cfg.AutoDelete),
		logger:   cfg.Logger,
	}, nil
}

// Publisher 实现 mqx.MessageQueue。从共享连接派生 Publisher。
func (mq *RabbitMQ) Publisher(opts ...mqx.PublishOption) (mqx.Publisher, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	pub, err := newPublisher(mq.conn, mq.topology, mq.logger, opts)
	if err != nil {
		return nil, fmt.Errorf("rabbitmqx: create publisher: %w", err)
	}
	return pub, nil
}

// Subscriber 实现 mqx.MessageQueue。从共享连接派生 Subscriber。
func (mq *RabbitMQ) Subscriber(opts ...mqx.SubscribeOption) (mqx.Subscriber, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	return newSubscriber(mq.conn, mq.topology, mq.logger, opts)
}

// Ping 实现 mqx.MessageQueue。检测 broker 连通性。
func (mq *RabbitMQ) Ping(ctx context.Context) error {
	return mq.conn.Ping(ctx)
}

// Close 实现 mqx.MessageQueue。关闭连接。
func (mq *RabbitMQ) Close() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil
	}
	mq.closed = true
	return mq.conn.Close()
}

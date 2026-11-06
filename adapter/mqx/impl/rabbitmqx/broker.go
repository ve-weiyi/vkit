package rabbitmqx

import (
	"context"
	"net"
	"net/url"
	"sync"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Broker 管理一个 AMQP 连接、发布器池与全部订阅。
type Broker struct {
	cfg      Config
	conn     *rabbitMQConn
	topology *topology
	pub      *publisher

	mu     sync.Mutex
	closed bool
	subs   map[*subscription]struct{}
}

var _ mqx.Broker = (*Broker)(nil)

// NewBroker 创建 Broker。LazyConnect 为 false 时立即拨号。
func NewBroker(cfg *Config) (*Broker, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.normalize()

	conn, err := dialConn(cfg)
	if err != nil {
		return nil, err
	}

	broker := &Broker{
		cfg:      *cfg,
		conn:     conn,
		topology: newTopology(cfg),
		subs:     make(map[*subscription]struct{}),
	}
	broker.pub = newPublisher(broker)
	return broker, nil
}

// Publisher 实现 mqx.Broker。
func (b *Broker) Publisher() (mqx.Publisher, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, mqx.ErrClosed
	}
	return b.pub, nil
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
	return b.conn.Ping(ctx)
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
	if err := b.conn.Close(); err != nil && firstErr == nil {
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

// BuildAMQPURL 拼装 AMQP 连接串，口令经 url.UserPassword 转义。
func BuildAMQPURL(host, port, username, password string) string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(username, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/",
	}
	return u.String()
}

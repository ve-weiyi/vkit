package kafkax

import (
	"context"
	"fmt"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Broker 管理一个 Kafka 生产者客户端与全部订阅。
type Broker struct {
	cfg      Config
	producer *kgo.Client

	mu     sync.Mutex
	closed bool
	subs   map[*subscription]struct{}
}

var _ mqx.Broker = (*Broker)(nil)

// NewBroker 创建 Broker。连接在首次请求时建立，不在构造期拨号。
func NewBroker(cfg *Config) (*Broker, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.normalize()

	producer, err := kgo.NewClient(cfg.producerOpts()...)
	if err != nil {
		return nil, fmt.Errorf("kafkax: create producer: %w", err)
	}
	return &Broker{cfg: *cfg, producer: producer, subs: make(map[*subscription]struct{})}, nil
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
	if err := b.producer.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
	}
	return nil
}

// Close 先停止全部订阅，再关闭生产者。
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
	b.producer.Close()
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

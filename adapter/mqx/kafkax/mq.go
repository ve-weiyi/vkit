package kafkax

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.MessageQueue = (*Kafka)(nil)

// Kafka Kafka 的 mqx.MessageQueue 实现。
// 通过 New() 创建，持有配置并从中派生 Publisher / Subscriber。
// Publisher 和 Subscriber 各自拥有独立的 kgo.Client。
type Kafka struct {
	mu     sync.Mutex
	cfg    Config
	closed bool
}

// New 创建 Kafka Kafka。仅在本地保存配置，不立即建立连接。
func New(cfg *Config) (*Kafka, error) {
	cfg.normalize()

	return &Kafka{cfg: *cfg}, nil
}

// Publisher 实现 mqx.MessageQueue。创建独立的 producer Client。
func (mq *Kafka) Publisher(opts ...mqx.PublishOption) (mqx.Publisher, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	pub, err := newPublisher(&mq.cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("kafka: create publisher: %w", err)
	}
	return pub, nil
}

// Subscriber 实现 mqx.MessageQueue。创建时仅保存默认配置，
// 实际的 kgo.Client 在 Subscribe 调用时创建。
func (mq *Kafka) Subscriber(opts ...mqx.SubscribeOption) (mqx.Subscriber, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil, mqx.ErrClosed
	}

	return newSubscriber(&mq.cfg, opts)
}

// Ping 实现 mqx.MessageQueue。尝试 TCP 连接任一 seed broker 以检测连通性。
func (mq *Kafka) Ping(ctx context.Context) error {
	if len(mq.cfg.Seeds) == 0 {
		return fmt.Errorf("%w: no seed brokers configured", mqx.ErrNotConnected)
	}

	// 使用配置的 DialTimeout，ctx deadline 优先
	dialTimeout := mq.cfg.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 5 * time.Second
	}
	dialer := &net.Dialer{Timeout: dialTimeout}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < dialer.Timeout {
			dialer.Timeout = remaining
		}
	}
	for _, seed := range mq.cfg.Seeds {
		conn, err := dialer.DialContext(ctx, "tcp", seed)
		if err == nil {
			_ = conn.Close()
			return nil
		}
	}
	return fmt.Errorf("%w: could not reach any seed broker", mqx.ErrNotConnected)
}

// Close 实现 mqx.MessageQueue。
func (mq *Kafka) Close() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.closed {
		return nil
	}
	mq.closed = true
	return nil
}

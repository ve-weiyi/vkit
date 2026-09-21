package rabbitmqx

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Publisher = (*Publisher)(nil)

// Publisher RabbitMQ 消息发布器。每条消息等待 Broker Confirm。
type Publisher struct {
	conn     *rabbitMQConn
	topology *topology
	mu       sync.Mutex
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	closed   bool
	reconnCh chan struct{}
	closeCh  chan struct{}
	logger   mqx.Logger

	defaultOpts []mqx.PublishOption
}

func newPublisher(conn *rabbitMQConn, topology *topology, logger mqx.Logger, opts []mqx.PublishOption) (*Publisher, error) {
	p := &Publisher{
		conn:        conn,
		topology:    topology,
		reconnCh:    conn.NotifyReconnect(),
		closeCh:     make(chan struct{}),
		logger:      logger,
		defaultOpts: opts,
	}
	if err := p.setupChannel(); err != nil {
		return nil, err
	}
	go p.reconnectWatch()
	return p, nil
}

// reconnectWatch 监听重连信号，重连后重建 AMQP channel。
func (p *Publisher) reconnectWatch() {
	for {
		select {
		case <-p.closeCh:
			return
		case <-p.conn.NotifyPermanentClose():
			return
		case _, ok := <-p.reconnCh:
			if !ok {
				return
			}
			// 提前检查 closed 状态，避免在 Close() 之后无效建连
			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				return
			}
			p.mu.Unlock()
			// 重连后重建 channel，失败忽略（下次重连再试）
			if err := p.setupChannel(); err != nil && p.logger != nil {
				p.logger.Error("mqx: rabbitmq publisher reconnect setup failed", "error", err.Error())
			}
		}
	}
}

func (p *Publisher) setupChannel() error {
	ch, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}

	if err := p.topology.Setup(ch); err != nil {
		ch.Close()
		return fmt.Errorf("setup topology: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return fmt.Errorf("enable confirm: %w", err)
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		ch.Close()
		return nil
	}
	// Close old channel before replacing, mirroring the subscriber pattern.
	if p.ch != nil {
		p.ch.Close()
	}
	p.ch = ch
	p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	p.mu.Unlock()
	return nil
}

// Publish 实现 mqx.Publisher。
func (p *Publisher) Publish(ctx context.Context, topic string, msg *mqx.Message, opts ...mqx.PublishOption) error {
	p.mu.Lock()
	ch := p.ch
	confirms := p.confirms
	p.mu.Unlock()

	if ch == nil {
		return mqx.ErrClosed
	}

	// 合并配置：实例级默认 + 调用级覆盖
	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Delay > 0 {
		return fmt.Errorf("%w: delayed publish requires rabbitmq-delayed-message-exchange plugin", mqx.ErrNotSupported)
	}

	// RabbitMQ 中 routing key 即消息路由目标，等价于其他后端的 topic/key。
	// WithKey 会覆盖 topic 参数，允许动态路由。
	routingKey := topic
	if cfg.Key != "" {
		routingKey = cfg.Key
	}
	pub := toAMQP(msg, cfg.Headers)

	if err := ch.PublishWithContext(ctx, p.topology.exchange, routingKey, false, false, pub); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrSendFailed, err)
	}

	// 等待 confirm
	if cfg.Timeout > 0 {
		timer := time.NewTimer(cfg.Timeout)
		defer timer.Stop()
		select {
		case confirm := <-confirms:
			if confirm.DeliveryTag == 0 {
				// channel 被关闭（连接断开），不是真正的 NACK
				return fmt.Errorf("%w: connection lost during confirm", mqx.ErrNotConnected)
			}
			if !confirm.Ack {
				return fmt.Errorf("%w: message nacked by broker", mqx.ErrSendFailed)
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return mqx.ErrTimeout
		}
	}
	select {
	case confirm := <-confirms:
		if confirm.DeliveryTag == 0 {
			return fmt.Errorf("%w: connection lost during confirm", mqx.ErrNotConnected)
		}
		if !confirm.Ack {
			return fmt.Errorf("%w: message nacked by broker", mqx.ErrSendFailed)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishBatch 实现 mqx.Publisher。
func (p *Publisher) PublishBatch(ctx context.Context, topic string, msgs []*mqx.Message, opts ...mqx.PublishOption) error {
	if len(msgs) == 0 {
		return nil
	}

	p.mu.Lock()
	ch := p.ch
	confirms := p.confirms
	p.mu.Unlock()

	if ch == nil {
		return mqx.ErrClosed
	}

	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	// RabbitMQ 中 routing key 即消息路由目标，等价于其他后端的 topic/key。
	routingKey := topic
	if cfg.Key != "" {
		routingKey = cfg.Key
	}

	for i, msg := range msgs {
		pub := toAMQP(msg, cfg.Headers)
		if err := ch.PublishWithContext(ctx, p.topology.exchange, routingKey, false, false, pub); err != nil {
			return fmt.Errorf("message %d/%d: %w: %v", i+1, len(msgs), mqx.ErrSendFailed, err)
		}
	}

	// 等待所有 confirm（整体超时，而非每条消息独立超时）
	var timeoutCh <-chan time.Time
	if cfg.Timeout > 0 {
		timer := time.NewTimer(cfg.Timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}
	for range msgs {
		select {
		case confirm := <-confirms:
			if confirm.DeliveryTag == 0 {
				return fmt.Errorf("%w: connection lost during confirm", mqx.ErrNotConnected)
			}
			if !confirm.Ack {
				return fmt.Errorf("%w: message nacked by broker", mqx.ErrSendFailed)
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			return mqx.ErrTimeout
		}
	}
	return nil
}

// Close 实现 mqx.Publisher。
func (p *Publisher) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// 通知 reconnectWatch 退出（在 p.mu 外执行，避免与 setupChannel 争锁）
	close(p.closeCh)

	// 关闭当前 channel
	p.mu.Lock()
	var err error
	if p.ch != nil {
		err = p.ch.Close()
		p.ch = nil
	}
	p.mu.Unlock()

	// 注销重连通知（持有 conn.mu，必须在 p.mu 外执行避免锁顺序冲突）
	p.conn.UnregisterReconnect(p.reconnCh)
	return err
}

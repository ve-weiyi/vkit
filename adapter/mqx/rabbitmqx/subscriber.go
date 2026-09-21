package rabbitmqx

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Subscriber = (*Subscriber)(nil)

// Subscriber RabbitMQ 消息消费器。
// Push 模型，内部消费循环将消息分发给 Handler。
// 支持自动重连：连接断开后等待重连信号，自动重建 channel 并恢复消费。
type Subscriber struct {
	conn     *rabbitMQConn
	topology *topology
	mu       sync.Mutex
	ch       *amqp.Channel
	closed   bool
	closeCh  chan struct{} // 通知重连循环退出
	reconnCh chan struct{} // 重连成功通知
	logger   mqx.Logger

	defaultOpts []mqx.SubscribeOption
}

func newSubscriber(conn *rabbitMQConn, topology *topology, logger mqx.Logger, opts []mqx.SubscribeOption) (*Subscriber, error) {
	return &Subscriber{
		conn:        conn,
		topology:    topology,
		closeCh:     make(chan struct{}),
		reconnCh:    conn.NotifyReconnect(),
		logger:      logger,
		defaultOpts: opts,
	}, nil
}

// Subscribe 实现 mqx.Subscriber。阻塞运行直到 ctx 被取消。
//
// 调用方必须通过取消 ctx 来停止消费。Close 不会中断正在运行的 Subscribe；
// 先取消 ctx 等待 Subscribe 返回，然后再调用 Close。
//
// 连接断开后自动重连并恢复消费，不会返回错误。
func (s *Subscriber) Subscribe(ctx context.Context, topic string, handler mqx.Handler, opts ...mqx.SubscribeOption) error {
	cfg := &mqx.SubscribeConfig{
		Concurrency: 1,
		Prefetch:    1,
		AutoAck:     true,
	}
	for _, o := range s.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Group == "" {
		return fmt.Errorf("%w: group is required for RabbitMQ subscriber", mqx.ErrBadMessage)
	}

	// 首次验证拓扑可达并获取队列名（使用临时 channel，验证后立即关闭）
	queueName, err := s.validateTopology(topic, cfg)
	if err != nil {
		return err
	}

	if cfg.Concurrency <= 1 {
		s.consumeSingleWithReconnect(ctx, topic, queueName, handler, cfg)
	} else {
		s.consumeConcurrentWithReconnect(ctx, topic, queueName, handler, cfg)
	}

	return nil
}

// consumeSingleWithReconnect 单消费者重连循环。
// 连接断开 → consumeLoop 返回 → 等待重连 → 重建 channel 并恢复消费。
func (s *Subscriber) consumeSingleWithReconnect(ctx context.Context, topic, queueName string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	for {
		if s.shouldExit(ctx) {
			return
		}

		ch, err := s.conn.Channel()
		if err != nil {
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq create consume channel failed", "error", err.Error())
			}
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}

		// 重连后重新声明拓扑（Exchange + Queue + Binding，幂等）
		if _, err := s.topology.SetupQueue(ch, cfg.Group, topic); err != nil {
			ch.Close()
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq setup topology failed", "error", err.Error())
			}
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}

		if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
			ch.Close()
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq set qos failed", "error", err.Error())
			}
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}

		deliveries, err := ch.Consume(
			queueName,
			"",    // consumer tag
			false, // always manual
			false, // exclusive
			false, // noLocal
			false, // noWait
			nil,   // args
		)
		if err != nil {
			ch.Close()
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq consume failed", "queue", queueName, "error", err.Error())
			}
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}

		// 更新 s.ch 引用，使 Close() 能正确关闭当前 channel
		s.mu.Lock()
		if s.ch != nil {
			s.ch.Close()
		}
		s.ch = ch
		s.mu.Unlock()

		s.consumeLoop(ctx, deliveries, handler, cfg)
		ch.Close()

		// consumeLoop 退出后判断原因
		if ctx.Err() != nil {
			return // 用户取消，正常退出
		}
		// 连接断开 → 等待重连后继续
		if !s.waitReconnect(ctx) {
			return
		}
	}
}

// consumeConcurrentWithReconnect 并发消费者重连循环。
// 连接断开 → 所有 worker 退出 → 等待重连 → 重新声明拓扑并启动 worker。
func (s *Subscriber) consumeConcurrentWithReconnect(ctx context.Context, topic, queueName string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	for {
		if s.shouldExit(ctx) {
			return
		}

		// 重连后需重新声明拓扑（Exchange + Queue + Binding 均幂等）
		topologyCh, err := s.conn.Channel()
		if err != nil {
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq create topology channel failed", "error", err.Error())
			}
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}
		if _, err := s.topology.SetupQueue(topologyCh, cfg.Group, topic); err != nil {
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq setup topology failed", "error", err.Error())
			}
			topologyCh.Close()
			if !s.waitReconnect(ctx) {
				return
			}
			continue
		}
		topologyCh.Close()

		// 消费循环阻塞直到所有 goroutine 退出（ctx 取消或连接断开）
		if err := s.consumeConcurrent(ctx, queueName, handler, cfg); err != nil {
			if s.logger != nil {
				s.logger.Error("mqx: rabbitmq concurrent consume failed", "error", err.Error())
			}
		}

		if ctx.Err() != nil {
			return
		}
		if !s.waitReconnect(ctx) {
			return
		}
	}
}

// shouldExit 检查是否应该退出重连循环。
func (s *Subscriber) shouldExit(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// waitReconnect 等待重连信号或退出信号。返回 true 表示收到重连信号，false 表示应退出。
func (s *Subscriber) waitReconnect(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.closeCh:
		return false
	case <-s.conn.NotifyPermanentClose():
		return false
	case _, ok := <-s.reconnCh:
		return ok
	}
}

// validateTopology 验证拓扑可达并返回队列名。
// 创建临时 channel 仅用于声明拓扑，验证后立即关闭，不存储到 s.ch。
// reconnect 循环会自行创建消费 channel。
func (s *Subscriber) validateTopology(topic string, cfg *mqx.SubscribeConfig) (string, error) {
	ch, err := s.conn.Channel()
	if err != nil {
		return "", fmt.Errorf("create channel: %w", err)
	}
	defer ch.Close()

	queueName, err := s.topology.SetupQueue(ch, cfg.Group, topic)
	if err != nil {
		return "", fmt.Errorf("setup topology: %w", err)
	}

	return queueName, nil
}

// consumeLoop 串行消费循环。
func (s *Subscriber) consumeLoop(ctx context.Context, deliveries <-chan amqp.Delivery, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			s.handleDelivery(ctx, d, handler, cfg)
		}
	}
}

// consumeConcurrent 并发消费：N 个独立 AMQP channel，各自 Consume。
// 每个 goroutine 拥有自己的 channel，避免并发调用 Ack/Nack 在同一 channel 上。
// 返回 error 当且仅当所有 goroutine 都启动失败时（至少一个成功启动则返回 nil）。
func (s *Subscriber) consumeConcurrent(ctx context.Context, queueName string, handler mqx.Handler, cfg *mqx.SubscribeConfig) error {
	var wg sync.WaitGroup
	var started atomic.Int32

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ch, err := s.conn.Channel()
			if err != nil {
				if s.logger != nil {
					s.logger.Error("mqx: rabbitmq concurrent worker channel failed",
						"worker", n,
						"error", err.Error(),
					)
				}
				return
			}
			defer ch.Close()

			if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
				if s.logger != nil {
					s.logger.Error("mqx: rabbitmq concurrent worker qos failed",
						"worker", n,
						"error", err.Error(),
					)
				}
				return
			}

			deliveries, err := ch.Consume(
				queueName,
				"",    // consumer tag
				false, // always manual
				false, // exclusive
				false, // noLocal
				false, // noWait
				nil,   // args
			)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("mqx: rabbitmq concurrent worker consume failed",
						"worker", n,
						"error", err.Error(),
					)
				}
				return
			}

			started.Add(1)
			s.consumeLoop(ctx, deliveries, handler, cfg)
		}(i)
	}
	wg.Wait()

	if started.Load() == 0 {
		return fmt.Errorf("%w: all concurrent consumers failed to start (queue=%s)", mqx.ErrSubscribeFailed, queueName)
	}
	return nil
}

// handleDelivery 处理单条消息：handler 返回 nil → Ack，error → 重试，耗尽 → DLQ + Nack。
func (s *Subscriber) handleDelivery(ctx context.Context, d amqp.Delivery, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	msg := fromAMQP(d)

	// manualAck 标记 handler 是否已通过 AckFn/NackFn 手动确认。
	// error 路径检测此标记以跳过 DLQ/Nack，避免覆盖 handler 的确认决策。
	manualAck := &mqx.ManualAck{}

	// AutoAck=false 时注入手动确认函数（Ack/Nack 由 mqx.ManualAck 保证只生效一次）
	if !cfg.AutoAck {
		manualAck.Inject(msg,
			func() error { return d.Ack(false) },
			func(requeue bool) error { return d.Nack(false, requeue) },
		)
	}

	err := handler(ctx, msg)
	if err == nil {
		if cfg.AutoAck && !manualAck.Manual() {
			_ = d.Ack(false)
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	// handler 已手动确认，跳过库的默认处理
	if manualAck.Manual() {
		return
	}

	// 重试耗尽
	if cfg.DLQTopic != "" {
		s.publishDLQ(msg, d.RoutingKey, err, cfg.DLQTopic)
	}

	// Nack 不 requeue：失败只影响重投递，必须记下来而不是静默丢弃
	if nackErr := d.Nack(false, false); nackErr != nil {
		logx.Errorf("rabbitmqx: nack message failed: %v", nackErr)
	}
}

// publishDLQ 将处理失败的消息发布到死信队列（best-effort）。
func (s *Subscriber) publishDLQ(msg *mqx.Message, originalKey string, handlerErr error, dlqTopic string) {
	dlqCh, chErr := s.conn.Channel()
	if chErr != nil {
		if s.logger != nil {
			s.logger.Error("mqx: rabbitmq DLQ channel creation failed",
				"dlq_topic", dlqTopic,
				"error", chErr.Error(),
			)
		}
		return
	}
	defer dlqCh.Close()

	dlqPub := toAMQP(msg, map[string]string{
		"original_routing_key": originalKey,
		"error":                handlerErr.Error(),
	})
	dlqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dlqCh.PublishWithContext(dlqCtx, s.topology.exchange, dlqTopic, false, false, dlqPub); err != nil {
		if s.logger != nil {
			s.logger.Error("mqx: rabbitmq DLQ publish failed",
				"dlq_topic", dlqTopic,
				"original_key", originalKey,
				"error", err.Error(),
			)
		}
	}
}

// Close 实现 mqx.Subscriber。
// Close 不会中断正在运行的 Subscribe。应取消 ctx 等待 Subscribe 返回后再调用 Close。
func (s *Subscriber) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// 通知重连循环退出（在 s.mu 外执行，避免与 consume 路径争锁）
	close(s.closeCh)

	// 关闭当前 channel
	s.mu.Lock()
	var err error
	if s.ch != nil {
		err = s.ch.Close()
		s.ch = nil
	}
	s.mu.Unlock()

	// 注销重连通知（持有 conn.mu，必须在 s.mu 外执行避免锁顺序冲突）
	s.conn.UnregisterReconnect(s.reconnCh)
	return err
}

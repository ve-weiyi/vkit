package rabbitmqx

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type subscriber struct {
	broker *Broker
}

var _ mqx.Subscriber = (*subscriber)(nil)

// Subscribe 实现 mqx.Subscriber。
func (s *subscriber) Subscribe(ctx context.Context, spec mqx.SubscriptionSpec, handler mqx.Handler) (mqx.Subscription, error) {
	prepared, err := spec.Prepare()
	if err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, fmt.Errorf("%w: handler is required", mqx.ErrBadMessage)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &subscription{
		broker:   s.broker,
		spec:     prepared,
		ctx:      subCtx,
		cancel:   cancel,
		done:     make(chan struct{}),
		errCh:    make(chan error, 1),
		channels: make(map[*amqp.Channel]struct{}),
	}
	sub.handler = mqx.Chain(
		mqx.MetricsMiddleware(s.broker.cfg.Metrics, prepared.Group),
	)(handler)

	if err := s.broker.register(sub); err != nil {
		cancel()
		return nil, err
	}

	// 先同步声明一次拓扑：outbox relay 可能在订阅启动后立即发布，
	// exchange 没有绑定时消息会被丢弃。失败时交给 worker 重试，不阻断启动。
	sub.declareTopology()

	for i := 0; i < prepared.Concurrency; i++ {
		sub.wg.Add(1)
		go sub.worker()
	}
	go func() {
		sub.wg.Wait()
		sub.finish()
	}()
	return sub, nil
}

type subscription struct {
	broker  *Broker
	spec    mqx.SubscriptionSpec
	handler mqx.Handler

	ctx    context.Context
	cancel context.CancelFunc

	wg    sync.WaitGroup
	done  chan struct{}
	errCh chan error

	closeOnce  sync.Once
	finishOnce sync.Once

	chMu     sync.Mutex
	channels map[*amqp.Channel]struct{}

	errMu    sync.Mutex
	fatalErr error
}

var _ mqx.Subscription = (*subscription)(nil)

func (s *subscription) Err() <-chan error { return s.errCh }

func (s *subscription) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	s.closeOnce.Do(func() {
		s.cancel()
		s.broker.unregister(s)
		select {
		case <-s.done:
		case <-ctx.Done():
			err = ctx.Err()
			s.closeChannels()
		}
	})
	return err
}

func (s *subscription) finish() {
	s.finishOnce.Do(func() {
		s.broker.unregister(s)
		close(s.errCh)
		close(s.done)
		if metrics := s.broker.cfg.Metrics; metrics != nil {
			metrics.SubscriptionStopped(s.spec.Destination, s.spec.Group, s.fatalError())
		}
	})
}

// declareTopology 在启动时同步声明主队列与死信队列；broker 未就绪时静默跳过。
func (s *subscription) declareTopology() {
	ch, err := s.broker.conn.Channel()
	if err != nil {
		return
	}
	defer func() { _ = ch.Close() }()

	if _, err := s.broker.topology.declareQueue(ch, s.spec.Destination, s.spec.Group, s.spec.BindingKey); err != nil {
		s.logf("mqx: rabbitmq initial topology declaration failed",
			"destination", s.spec.Destination,
			"queue", s.spec.Group,
			"error", err.Error(),
		)
		return
	}
	if s.spec.DeadLetter.Enabled {
		if _, err := s.broker.topology.declareQueue(ch, s.spec.DeadLetter.Destination, s.spec.Group+".dlq", s.spec.DeadLetter.RoutingKey); err != nil {
			s.logf("mqx: rabbitmq initial dead-letter topology declaration failed",
				"destination", s.spec.DeadLetter.Destination,
				"queue", s.spec.Group+".dlq",
				"error", err.Error(),
			)
		}
	}
}

func (s *subscription) worker() {
	defer s.wg.Done()

	reconn := s.broker.conn.NotifyReconnect()
	defer s.broker.conn.UnregisterReconnect(reconn)

	for {
		if s.ctx.Err() != nil {
			return
		}

		ch, err := s.broker.conn.Channel()
		if err != nil {
			s.logf("mqx: rabbitmq create consume channel failed", "error", err.Error())
			if !s.waitReconnect(reconn) {
				return
			}
			continue
		}

		queue, err := s.broker.topology.declareQueue(ch, s.spec.Destination, s.spec.Group, s.spec.BindingKey)
		if err != nil {
			_ = ch.Close()
			s.logf("mqx: rabbitmq setup topology failed",
				"destination", s.spec.Destination,
				"queue", s.spec.Group,
				"error", err.Error(),
			)
			if !s.waitReconnect(reconn) {
				return
			}
			continue
		}

		// 预先声明死信队列，避免死信 exchange 没有绑定而直接丢弃消息。
		if s.spec.DeadLetter.Enabled {
			if _, err := s.broker.topology.declareQueue(
				ch,
				s.spec.DeadLetter.Destination,
				s.spec.Group+".dlq",
				s.spec.DeadLetter.RoutingKey,
			); err != nil {
				_ = ch.Close()
				s.logf("mqx: rabbitmq setup dead-letter topology failed",
					"destination", s.spec.DeadLetter.Destination,
					"queue", s.spec.Group+".dlq",
					"error", err.Error(),
				)
				if !s.waitReconnect(reconn) {
					return
				}
				continue
			}
		}

		if err := ch.Qos(prefetchFor(s.spec), 0, false); err != nil {
			_ = ch.Close()
			s.logf("mqx: rabbitmq set qos failed", "queue", queue, "error", err.Error())
			if !s.waitReconnect(reconn) {
				return
			}
			continue
		}

		deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			s.logf("mqx: rabbitmq consume failed", "queue", queue, "error", err.Error())
			if !s.waitReconnect(reconn) {
				return
			}
			continue
		}

		s.track(ch)
		s.consume(deliveries)
		s.untrack(ch)
		_ = ch.Close()

		if s.ctx.Err() != nil {
			return
		}
		if !s.waitReconnect(reconn) {
			return
		}
	}
}

func (s *subscription) consume(deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			s.handle(d)
		}
	}
}

func (s *subscription) handle(d amqp.Delivery) {
	delivery := mqx.NewDelivery(
		fromAMQP(s.spec.Destination, d),
		func() error { return d.Ack(false) },
		func(requeue bool) error { return d.Nack(false, requeue) },
	)
	delivery.Redelivered = d.Redelivered

	ctx := s.ctx
	if s.spec.HandlerTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.spec.HandlerTimeout)
		defer cancel()
	}

	err := mqx.Process(ctx, delivery, s.handler, s.spec.Retry)
	if delivery.Settled() {
		return
	}
	if err == nil {
		_ = d.Ack(false)
		return
	}
	if s.ctx.Err() != nil {
		return
	}

	if s.spec.DeadLetter.Enabled {
		if dlqErr := s.deadLetter(delivery, err); dlqErr != nil {
			s.logf("mqx: rabbitmq dead-letter publish failed",
				"destination", delivery.Destination,
				"routing_key", delivery.RoutingKey,
				"message_id", delivery.ID,
				"dead_letter", s.spec.DeadLetter.Destination,
				"error", dlqErr.Error(),
			)
			// 不确认也不立即 requeue：停止订阅，消息在 channel 关闭后由 broker 重投，
			// 避免死信不可用时无限热循环。
			s.fail(fmt.Errorf("mqx: rabbitmq dead-letter: %w", dlqErr))
			return
		}
		s.logf("mqx: rabbitmq message dead-lettered",
			"destination", delivery.Destination,
			"routing_key", delivery.RoutingKey,
			"message_id", delivery.ID,
			"dead_letter", s.spec.DeadLetter.Destination,
			"attempts", delivery.Attempt,
			"error", err.Error(),
		)
		_ = d.Ack(false)
		return
	}

	s.logf("mqx: rabbitmq message discarded after retries",
		"destination", delivery.Destination,
		"routing_key", delivery.RoutingKey,
		"message_id", delivery.ID,
		"attempts", delivery.Attempt,
		"error", err.Error(),
	)
	_ = d.Ack(false)
}

func (s *subscription) deadLetter(d *mqx.Delivery, cause error) error {
	policy := s.spec.DeadLetter
	// 保留原事件 ID：死信重放仍按原事件去重。
	dlq := d.Message.Clone()
	dlq.Destination = policy.Destination
	if policy.RoutingKey != "" {
		dlq.RoutingKey = policy.RoutingKey
	}
	if policy.IncludeMeta {
		if dlq.Headers == nil {
			dlq.Headers = make(map[string]string, 6)
		}
		dlq.Headers[mqx.HeaderOriginalDestination] = d.Destination
		dlq.Headers[mqx.HeaderOriginalRoutingKey] = d.RoutingKey
		dlq.Headers[mqx.HeaderOriginalGroup] = s.spec.Group
		dlq.Headers[mqx.HeaderAttempts] = strconv.Itoa(d.Attempt)
		dlq.Headers[mqx.HeaderError] = cause.Error()
		dlq.Headers[mqx.HeaderFailedAt] = time.Now().Format(time.RFC3339Nano)
	}

	prepared, err := dlq.Prepared()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.broker.pub.Publish(ctx, &prepared)
	return err
}

func (s *subscription) waitReconnect(reconn <-chan struct{}) bool {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-s.ctx.Done():
		return false
	case <-s.broker.conn.NotifyPermanentClose():
		s.report(fmt.Errorf("%w: connection permanently closed", mqx.ErrUnavailable))
		return false
	case _, ok := <-reconn:
		return ok
	case <-timer.C:
		// 连接仍在但 channel/拓扑出错时，不依赖重连信号，按固定间隔重试。
		return true
	}
}

func (s *subscription) report(err error) {
	s.errMu.Lock()
	if s.fatalErr == nil {
		s.fatalErr = err
	}
	s.errMu.Unlock()

	select {
	case s.errCh <- err:
	default:
	}
}

func (s *subscription) fatalError() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.fatalErr
}

func (s *subscription) fail(err error) {
	s.report(err)
	s.cancel()
}

func (s *subscription) track(ch *amqp.Channel) {
	s.chMu.Lock()
	s.channels[ch] = struct{}{}
	s.chMu.Unlock()
}

func (s *subscription) untrack(ch *amqp.Channel) {
	s.chMu.Lock()
	delete(s.channels, ch)
	s.chMu.Unlock()
}

func (s *subscription) closeChannels() {
	s.chMu.Lock()
	defer s.chMu.Unlock()
	for ch := range s.channels {
		_ = ch.Close()
	}
}

func (s *subscription) logf(msg string, args ...any) {
	s.broker.cfg.Logger.Error(msg, args...)
}

func prefetchFor(spec mqx.SubscriptionSpec) int {
	prefetch := spec.MaxInFlight / spec.Concurrency
	if prefetch < 1 {
		prefetch = 1
	}
	return prefetch
}

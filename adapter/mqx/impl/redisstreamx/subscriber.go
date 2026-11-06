package redisstreamx

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type subscriber struct {
	broker *Broker
}

var _ mqx.Subscriber = (*subscriber)(nil)

// Subscribe 实现 mqx.Subscriber。
//
// 所有消费者共享同一个 stream 与 group；XREADGROUP 负责组内分摊，
// Nack(requeue=true) 通过重新 XADD 实现立即重投，进程崩溃留下的 pending
// 由 ClaimIdle 触发的 XAUTOCLAIM 回收。
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

	consumer := fmt.Sprintf("%s-%s", hostname(), uuid.NewString())
	subCtx, cancel := context.WithCancel(ctx)
	sub := &subscription{
		broker:     s.broker,
		spec:       prepared,
		stream:     prepared.Destination,
		group:      prepared.Group,
		consumer:   consumer,
		binding:    prepared.BindingKey,
		ctx:        subCtx,
		cancel:     cancel,
		deliveries: make(chan redis.XMessage),
		inflight:   make(chan struct{}, prepared.MaxInFlight),
		done:       make(chan struct{}),
		errCh:      make(chan error, 1),
	}
	sub.handler = mqx.Chain(
		mqx.MetricsMiddleware(s.broker.cfg.Metrics, prepared.Group),
	)(handler)

	if prepared.DeadLetter.Enabled {
		pub, err := s.broker.Publisher()
		if err != nil {
			cancel()
			return nil, err
		}
		sub.dlq = pub
	}

	groupCtx, groupCancel := context.WithTimeout(subCtx, 5*time.Second)
	err = s.broker.client.XGroupCreateMkStream(groupCtx, prepared.Destination, prepared.Group, "0").Err()
	groupCancel()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		cancel()
		return nil, fmt.Errorf("%w: create consumer group: %v", mqx.ErrUnavailable, err)
	}

	if err := s.broker.register(sub); err != nil {
		cancel()
		return nil, err
	}

	sub.wg.Add(1)
	go sub.consumeLoop()
	if sub.broker.cfg.ClaimIdle > 0 {
		sub.wg.Add(1)
		go sub.claimLoop()
	}
	for i := 0; i < prepared.Concurrency; i++ {
		sub.wg.Add(1)
		go sub.work()
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
	dlq     mqx.Publisher

	stream   string
	group    string
	consumer string
	binding  string

	ctx    context.Context
	cancel context.CancelFunc

	// deliveries 无缓冲：在途消息数由 inflight 令牌控制在 MaxInFlight 以内。
	deliveries chan redis.XMessage
	inflight   chan struct{}
	wg         sync.WaitGroup

	done       chan struct{}
	errCh      chan error
	finishOnce sync.Once
	closeOnce  sync.Once
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
		select {
		case <-s.done:
		case <-ctx.Done():
			err = ctx.Err()
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
			metrics.SubscriptionStopped(s.spec.Destination, s.spec.Group, nil)
		}
	})
}

func (s *subscription) consumeLoop() {
	defer s.wg.Done()

	for {
		if s.ctx.Err() != nil {
			return
		}

		result, err := s.broker.client.XReadGroup(s.ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Count:    s.broker.cfg.ReadCount,
			Block:    s.broker.cfg.BlockTimeout,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			s.logf("mqx: redis XReadGroup failed", "stream", s.stream, "error", err.Error())
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		for _, stream := range result {
			for _, msg := range stream.Messages {
				if !s.dispatch(msg) {
					return
				}
			}
		}
	}
}

// claimLoop 周期性接管闲置超过 ClaimIdle 的待确认消息。
func (s *subscription) claimLoop() {
	defer s.wg.Done()

	interval := s.broker.cfg.ClaimIdle / 2
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	if interval > time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		s.claimPending()
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *subscription) claimPending() {
	if s.broker.cfg.ClaimIdle <= 0 {
		return
	}
	start := "0-0"
	for {
		if s.ctx.Err() != nil {
			return
		}
		messages, next, err := s.broker.client.XAutoClaim(s.ctx, &redis.XAutoClaimArgs{
			Stream:   s.stream,
			Group:    s.group,
			Consumer: s.consumer,
			MinIdle:  s.broker.cfg.ClaimIdle,
			Start:    start,
			Count:    s.broker.cfg.ReadCount,
		}).Result()
		if err != nil {
			if s.ctx.Err() == nil {
				s.logf("mqx: redis XAutoClaim failed", "stream", s.stream, "error", err.Error())
			}
			return
		}
		for _, msg := range messages {
			if !s.dispatch(msg) {
				return
			}
		}
		if next == "0-0" {
			return
		}
		start = next
	}
}

func (s *subscription) work() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.deliveries:
			s.handle(msg)
			<-s.inflight
		}
	}
}

// dispatch 取一个在途名额后把消息交给 worker；订阅关闭时返回 false。
// 名额在 handler 返回后释放，因此未确认消息数不会超过 MaxInFlight。
func (s *subscription) dispatch(msg redis.XMessage) bool {
	select {
	case s.inflight <- struct{}{}:
	case <-s.ctx.Done():
		return false
	}
	select {
	case s.deliveries <- msg:
		return true
	case <-s.ctx.Done():
		<-s.inflight
		return false
	}
}

func (s *subscription) handle(xMsg redis.XMessage) {
	msg := fromRedis(s.stream, xMsg.ID, xMsg.Values)
	if s.binding != "" && !mqx.MatchTopic(s.binding, msg.RoutingKey) {
		s.ack(xMsg.ID)
		return
	}

	delivery := mqx.NewDelivery(msg,
		func() error { return s.ack(xMsg.ID) },
		func(requeue bool) error {
			if !requeue {
				return s.ack(xMsg.ID)
			}
			if err := s.requeue(xMsg); err != nil {
				s.logf("mqx: redis requeue failed",
					"stream", s.stream,
					"message_id", xMsg.ID,
					"error", err.Error(),
				)
				return err
			}
			return s.ack(xMsg.ID)
		},
	)

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
		_ = s.ack(xMsg.ID)
		return
	}
	if s.ctx.Err() != nil {
		// 留在 PEL，等待下次启动或 XAUTOCLAIM 回收。
		return
	}

	if s.spec.DeadLetter.Enabled {
		if dlqErr := s.deadLetter(delivery, err); dlqErr != nil {
			s.logf("mqx: redis dead-letter publish failed",
				"stream", s.stream,
				"group", s.group,
				"message_id", delivery.ID,
				"dead_letter", s.spec.DeadLetter.Destination,
				"error", dlqErr.Error(),
			)
			// 不确认：留在 PEL，由 ClaimIdle 回收后重投。
			return
		}
		s.logf("mqx: redis message dead-lettered",
			"stream", s.stream,
			"group", s.group,
			"routing_key", delivery.RoutingKey,
			"message_id", delivery.ID,
			"dead_letter", s.spec.DeadLetter.Destination,
			"attempts", delivery.Attempt,
			"error", err.Error(),
		)
		_ = s.ack(xMsg.ID)
		return
	}

	s.logf("mqx: redis message discarded after retries",
		"stream", s.stream,
		"group", s.group,
		"routing_key", delivery.RoutingKey,
		"message_id", delivery.ID,
		"attempts", delivery.Attempt,
		"error", err.Error(),
	)
	_ = s.ack(xMsg.ID)
}

// requeue 复制一条消息重新 XADD，再确认原消息。
func (s *subscription) requeue(xMsg redis.XMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.broker.client.XAdd(ctx, &redis.XAddArgs{Stream: s.stream, Values: xMsg.Values}).Err()
}

func (s *subscription) deadLetter(d *mqx.Delivery, cause error) error {
	policy := s.spec.DeadLetter
	if s.dlq == nil {
		return fmt.Errorf("%w: dead letter publisher is nil", mqx.ErrUnavailable)
	}

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
	_, err = s.dlq.Publish(ctx, &prepared)
	return err
}

func (s *subscription) ack(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.broker.client.XAck(ctx, s.stream, s.group, id).Err(); err != nil {
		s.logf("mqx: redis XAck failed",
			"stream", s.stream,
			"group", s.group,
			"stream_id", id,
			"error", err.Error(),
		)
		return fmt.Errorf("%w: xack: %v", mqx.ErrUnavailable, err)
	}
	return nil
}

func (s *subscription) logf(msg string, args ...any) {
	s.broker.cfg.Logger.Error(msg, args...)
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "consumer"
	}
	return name
}

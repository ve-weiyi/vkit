// Package memmqx 是 mqx.Broker 的内存实现，用于测试与本地开发。
//
// 语义：
//   - 同一 Destination 下，同组订阅相互竞争（每条消息只投给组内一个订阅），
//     不同组各自收到一份；
//   - Ack/Nack 不持久化；Nack(requeue=true) 立即重投，队列满时记录日志并丢弃；
//   - DLQ 通过向 DeadLetter.Destination 发布消息模拟，没有消费者时消息被丢弃；
//   - 不支持分区、持久化与重平衡。
package memmqx

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var (
	_ mqx.Broker         = (*Broker)(nil)
	_ mqx.Publisher      = (*publisher)(nil)
	_ mqx.BatchPublisher = (*publisher)(nil)
	_ mqx.Subscriber     = (*subscriber)(nil)
	_ mqx.Subscription   = (*subscription)(nil)
)

// Config 配置内存 Broker。
type Config struct {
	// QueueSize 覆盖订阅的待处理队列长度；0 表示使用 SubscriptionSpec.MaxInFlight。
	QueueSize int
	Logger    *slog.Logger
	Metrics   mqx.Metrics
}

// Broker 是内存消息总线。
type Broker struct {
	mu     sync.Mutex
	cfg    Config
	closed bool
	subs   map[*subscription]struct{}
	groups map[string]*groupState
}

type groupState struct {
	subs []*subscription
	next int
}

// NewBroker 创建内存 Broker。
func NewBroker(cfg Config) *Broker {
	return &Broker{
		cfg:    cfg,
		subs:   make(map[*subscription]struct{}),
		groups: make(map[string]*groupState),
	}
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
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return mqx.ErrClosed
	}
	return nil
}

// Close 实现 mqx.Broker：先停止全部订阅，再标记关闭。
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
	return firstErr
}

func (b *Broker) register(sub *subscription) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return mqx.ErrClosed
	}
	b.subs[sub] = struct{}{}

	key := groupKey(sub.spec.Destination, sub.spec.Group)
	group := b.groups[key]
	if group == nil {
		group = &groupState{}
		b.groups[key] = group
	}
	group.subs = append(group.subs, sub)
	return nil
}

func (b *Broker) unregister(sub *subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, sub)

	key := groupKey(sub.spec.Destination, sub.spec.Group)
	group := b.groups[key]
	if group == nil {
		return
	}
	for i, registered := range group.subs {
		if registered == sub {
			group.subs = append(group.subs[:i], group.subs[i+1:]...)
			break
		}
	}
	if len(group.subs) == 0 {
		delete(b.groups, key)
	}
}

// route 按「每个消费组一份」的语义选择订阅并投递。
func (b *Broker) route(ctx context.Context, msg *mqx.Message) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return mqx.ErrClosed
	}
	targets := make([]*subscription, 0, len(b.groups))
	for _, group := range b.groups {
		candidates := group.match(msg)
		if len(candidates) == 0 {
			continue
		}
		index := group.next % len(candidates)
		group.next++
		targets = append(targets, candidates[index])
	}
	b.mu.Unlock()

	for _, sub := range targets {
		if err := sub.enqueue(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (g *groupState) match(msg *mqx.Message) []*subscription {
	candidates := make([]*subscription, 0, len(g.subs))
	for _, sub := range g.subs {
		if sub.matches(msg) {
			candidates = append(candidates, sub)
		}
	}
	return candidates
}

func groupKey(destination, group string) string {
	return destination + "\x00" + group
}

type publisher struct {
	broker *Broker
}

func (p *publisher) Publish(ctx context.Context, msg *mqx.Message) (*mqx.PublishResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", mqx.ErrBadMessage)
	}
	prepared, err := msg.Prepared()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = p.broker.route(ctx, &prepared)
	if metrics := p.broker.cfg.Metrics; metrics != nil {
		metrics.PublishCompleted(prepared.Destination, prepared.RoutingKey, time.Since(start), err)
	}
	if err != nil {
		return nil, err
	}
	return &mqx.PublishResult{ID: prepared.ID}, nil
}

func (p *publisher) PublishBatch(ctx context.Context, msgs []*mqx.Message) ([]mqx.PublishResult, error) {
	results := make([]mqx.PublishResult, 0, len(msgs))
	for _, msg := range msgs {
		result, err := p.Publish(ctx, msg)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
	}
	return results, nil
}

type subscriber struct {
	broker *Broker
}

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
		broker: s.broker,
		spec:   prepared,
		ctx:    subCtx,
		cancel: cancel,
		queue:  make(chan *mqx.Delivery, queueSize(prepared, s.broker.cfg.QueueSize)),
		done:   make(chan struct{}),
		errCh:  make(chan error, 1),
	}
	sub.handler = mqx.Chain(
		mqx.MetricsMiddleware(s.broker.cfg.Metrics, prepared.Group),
	)(handler)

	if err := s.broker.register(sub); err != nil {
		cancel()
		return nil, err
	}

	for i := 0; i < prepared.Concurrency; i++ {
		sub.wg.Add(1)
		go sub.run()
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

	queue chan *mqx.Delivery
	wg    sync.WaitGroup

	done       chan struct{}
	errCh      chan error
	finishOnce sync.Once
	closeOnce  sync.Once
}

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

func (s *subscription) run() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case d := <-s.queue:
			s.handle(d)
		}
	}
}

func (s *subscription) matches(msg *mqx.Message) bool {
	if s.spec.Destination != msg.Destination {
		return false
	}
	binding := s.spec.BindingKey
	if binding == "" {
		binding = "#"
	}
	return mqx.MatchTopic(binding, msg.RoutingKey)
}

func (s *subscription) enqueue(ctx context.Context, msg *mqx.Message) error {
	d := s.newDelivery(msg)
	select {
	case s.queue <- d:
		return nil
	case <-s.ctx.Done():
		return mqx.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *subscription) newDelivery(msg *mqx.Message) *mqx.Delivery {
	clone := msg.Clone()
	var requeue func()
	requeue = func() { s.requeue(&clone) }
	return mqx.NewDelivery(clone, nil, func(retry bool) error {
		if retry {
			requeue()
		}
		return nil
	})
}

// requeue 非阻塞重投：队列满时记录日志并丢弃，避免 handler 内 Nack(true) 自锁。
func (s *subscription) requeue(msg *mqx.Message) {
	select {
	case s.queue <- s.newDelivery(msg):
	default:
		s.logf("mqx: mem requeue dropped, queue is full",
			"destination", msg.Destination,
			"routing_key", msg.RoutingKey,
			"message_id", msg.ID,
		)
	}
}

func (s *subscription) handle(d *mqx.Delivery) {
	ctx := s.ctx
	if s.spec.HandlerTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.spec.HandlerTimeout)
		defer cancel()
	}

	err := mqx.Process(ctx, d, s.handler, s.spec.Retry)
	if d.Settled() {
		return
	}
	if err == nil {
		_ = d.Ack()
		return
	}
	if s.ctx.Err() != nil {
		return
	}

	if s.spec.DeadLetter.Enabled {
		if dlqErr := s.deadLetter(d, err); dlqErr != nil {
			s.logf("mqx: mem dead-letter publish failed",
				"destination", d.Destination,
				"routing_key", d.RoutingKey,
				"message_id", d.ID,
				"error", dlqErr.Error(),
			)
			// 死信写入失败不确认原消息：尽力重投，队列满时由 requeue 记录丢弃。
			s.requeue(&d.Message)
			return
		}
		s.logf("mqx: mem message dead-lettered",
			"destination", d.Destination,
			"routing_key", d.RoutingKey,
			"message_id", d.ID,
			"dead_letter", s.spec.DeadLetter.Destination,
			"attempts", d.Attempt,
			"error", err.Error(),
		)
		_ = d.Ack()
		return
	}

	s.logf("mqx: mem message discarded after retries",
		"destination", d.Destination,
		"routing_key", d.RoutingKey,
		"message_id", d.ID,
		"attempts", d.Attempt,
		"error", err.Error(),
	)
	_ = d.Nack(false)
}

func (s *subscription) deadLetter(d *mqx.Delivery, cause error) error {
	// 保留原事件 ID：死信重放仍按原事件去重。
	dlq := d.Message.Clone()
	dlq.Destination = s.spec.DeadLetter.Destination
	if s.spec.DeadLetter.RoutingKey != "" {
		dlq.RoutingKey = s.spec.DeadLetter.RoutingKey
	}
	if s.spec.DeadLetter.IncludeMeta {
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
	return s.broker.route(ctx, &prepared)
}

func (s *subscription) logf(msg string, args ...any) {
	if s.broker.cfg.Logger != nil {
		s.broker.cfg.Logger.Error(msg, args...)
	}
}

func queueSize(spec mqx.SubscriptionSpec, configured int) int {
	size := spec.MaxInFlight
	if configured > 0 {
		size = configured
	}
	if size < 1 {
		size = 1
	}
	return size
}

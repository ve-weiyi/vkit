package kafkax

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type subscriber struct {
	broker *Broker
}

var _ mqx.Subscriber = (*subscriber)(nil)

// Subscribe 实现 mqx.Subscriber。
//
// Concurrency 表示同一 consumer group 内的客户端数量；每个客户端负责被分配的
// partition，并在单 loop 内顺序处理，因此按 record 提交 offset 是安全的。
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
		done:   make(chan struct{}),
		errCh:  make(chan error, 1),
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

	for i := 0; i < prepared.Concurrency; i++ {
		client, err := kgo.NewClient(s.broker.cfg.consumerOpts(prepared.Group, prepared.Destination)...)
		if err != nil {
			cancel()
			for _, created := range sub.clients {
				created.Close()
			}
			return nil, fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
		}
		sub.clients = append(sub.clients, client)
	}

	if err := s.broker.register(sub); err != nil {
		cancel()
		for _, client := range sub.clients {
			client.Close()
		}
		return nil, err
	}

	for _, client := range sub.clients {
		sub.wg.Add(1)
		go sub.consume(client)
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

	ctx    context.Context
	cancel context.CancelFunc

	clients []*kgo.Client
	wg      sync.WaitGroup

	done       chan struct{}
	errCh      chan error
	finishOnce sync.Once
	closeOnce  sync.Once

	closeClientsOnce sync.Once
	errMu            sync.Mutex
	fatalErr         error
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
		s.closeClients()
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
		s.closeClients()
		close(s.errCh)
		close(s.done)
		if metrics := s.broker.cfg.Metrics; metrics != nil {
			metrics.SubscriptionStopped(s.spec.Destination, s.spec.Group, s.fatalError())
		}
	})
}

// closeClients 关闭全部消费者；异常结束与正常关闭共用，避免 client 泄漏。
func (s *subscription) closeClients() {
	s.closeClientsOnce.Do(func() {
		for _, client := range s.clients {
			client.Close()
		}
	})
}

func (s *subscription) consume(client *kgo.Client) {
	defer s.wg.Done()

	for {
		if s.ctx.Err() != nil {
			return
		}

		fetches := client.PollFetches(s.ctx)
		if s.ctx.Err() != nil || fetches.IsClientClosed() {
			return
		}
		for _, fetchErr := range fetches.Errors() {
			s.logf("mqx: kafka fetch error",
				"topic", fetchErr.Topic,
				"partition", fetchErr.Partition,
				"error", fetchErr.Err.Error(),
			)
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			if s.ctx.Err() != nil {
				return
			}
			s.handle(client, iter.Next())
		}
	}
}

func (s *subscription) handle(client *kgo.Client, record *kgo.Record) {
	msg := fromRecord(record)
	if s.spec.BindingKey != "" && !mqx.MatchTopic(s.spec.BindingKey, msg.RoutingKey) {
		_ = s.commit(client, record)
		return
	}

	delivery := mqx.NewDelivery(msg,
		func() error { return s.commit(client, record) },
		func(requeue bool) error {
			if !requeue {
				return s.commit(client, record)
			}
			if err := s.requeue(record); err != nil {
				s.fail(fmt.Errorf("mq: kafka requeue: %w", err))
				return err
			}
			return s.commit(client, record)
		},
	)
	delivery.Partition = record.Partition
	delivery.Offset = record.Offset

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
		_ = s.commit(client, record)
		return
	}
	if s.ctx.Err() != nil {
		// 不提交，交给重平衡/重启后的重投。
		return
	}

	if s.spec.DeadLetter.Enabled {
		if dlqErr := s.deadLetter(delivery, err); dlqErr != nil {
			s.logf("mqx: kafka dead-letter publish failed",
				"topic", record.Topic,
				"partition", record.Partition,
				"offset", record.Offset,
				"message_id", delivery.ID,
				"error", dlqErr.Error(),
			)
			// 不提交并停止订阅：继续消费会让后续 offset 提交越过这条消息。
			s.fail(fmt.Errorf("mq: kafka dead-letter: %w", dlqErr))
			return
		}
		s.logf("mqx: kafka message dead-lettered",
			"topic", record.Topic,
			"partition", record.Partition,
			"offset", record.Offset,
			"routing_key", delivery.RoutingKey,
			"message_id", delivery.ID,
			"dead_letter", s.spec.DeadLetter.Destination,
			"attempts", delivery.Attempt,
			"error", err.Error(),
		)
		_ = s.commit(client, record)
		return
	}

	s.logf("mqx: kafka message discarded after retries",
		"topic", record.Topic,
		"partition", record.Partition,
		"offset", record.Offset,
		"routing_key", delivery.RoutingKey,
		"message_id", delivery.ID,
		"attempts", delivery.Attempt,
		"error", err.Error(),
	)
	_ = s.commit(client, record)
}

// requeue 复制一条记录重新投递，再提交原 offset。Kafka 无法对单条消息 requeue。
func (s *subscription) requeue(record *kgo.Record) error {
	copied := *record
	copied.Headers = append([]kgo.RecordHeader(nil), record.Headers...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.broker.producer.ProduceSync(ctx, &copied).FirstErr()
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

func (s *subscription) commit(client *kgo.Client, record *kgo.Record) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.CommitRecords(ctx, record); err != nil {
		s.logf("mqx: kafka commit failed",
			"topic", record.Topic,
			"partition", record.Partition,
			"offset", record.Offset,
			"error", err.Error(),
		)
		return fmt.Errorf("%w: commit: %v", mqx.ErrUnavailable, err)
	}
	return nil
}

func (s *subscription) fail(err error) {
	s.report(err)
	s.cancel()
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

func (s *subscription) logf(msg string, args ...any) {
	s.broker.cfg.Logger.Error(msg, args...)
}

package kafkax

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Subscriber = (*Subscriber)(nil)

// Subscriber Kafka 消息消费器。
// 基于 Consumer Group + PollFetches 实现消费循环。
type Subscriber struct {
	mu     sync.Mutex
	cfg    *Config
	client *kgo.Client
	closed bool
	logger mqx.Logger

	defaultOpts []mqx.SubscribeOption
}

func newSubscriber(cfg *Config, opts []mqx.SubscribeOption) (*Subscriber, error) {
	return &Subscriber{
		cfg:         cfg,
		logger:      cfg.Logger,
		defaultOpts: opts,
	}, nil
}

// Subscribe 实现 mqx.Subscriber。阻塞运行直到 ctx 被取消。
//
// 调用方必须通过取消 ctx 来停止消费。Close 不会中断正在运行的 Subscribe；
// 先取消 ctx 等待 Subscribe 返回，然后再调用 Close。
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
		return fmt.Errorf("%w: group is required for Kafka subscriber", mqx.ErrBadMessage)
	}

	client, err := s.createConsumerClient(topic, cfg)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		client.Close()
		return mqx.ErrClosed
	}
	if s.client != nil {
		s.client.Close()
	}
	s.client = client
	s.mu.Unlock()

	if cfg.Concurrency <= 1 {
		s.consumeLoop(ctx, client, topic, cfg.Group, handler, cfg)
	} else {
		s.consumeConcurrent(ctx, client, topic, cfg.Group, handler, cfg)
	}

	return nil
}

// createConsumerClient 创建配置了 Consumer Group 的 kgo.Client。
func (s *Subscriber) createConsumerClient(topic string, cfg *mqx.SubscribeConfig) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(s.cfg.Seeds...),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.FetchMaxWait(1 * time.Second),
		kgo.MaxBufferedRecords(cfg.Prefetch * max(cfg.Concurrency, 1)),
	}

	if s.cfg.TLS != nil {
		opts = append(opts, kgo.DialTLSConfig(s.cfg.TLS))
	}
	if s.cfg.SASL != nil {
		if m := s.cfg.SASL.mechanism(); m != nil {
			opts = append(opts, m)
		}
	}
	if s.cfg.DialTimeout > 0 {
		opts = append(opts, kgo.DialTimeout(s.cfg.DialTimeout))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: create consumer client: %v", mqx.ErrSubscribeFailed, err)
	}

	// 验证连通性
	if err := client.Ping(context.Background()); err != nil {
		client.Close()
		return nil, fmt.Errorf("%w: %v", mqx.ErrNotConnected, err)
	}

	return client, nil
}

// consumeLoop 串行消费循环。PollFetches → 逐条处理 → 提交 offset。
func (s *Subscriber) consumeLoop(ctx context.Context, client *kgo.Client, topic, group string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	for {
		select {
		case <-ctx.Done():
			s.commitOnExit(client, ctx)
			return
		default:
		}

		fetches := client.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			if ctx.Err() != nil {
				s.commitOnExit(client, ctx)
			}
			return
		}

		// 记录 Fetch 层面的错误（如分区无 leader），但不影响已拉取到的记录
		if s.logger != nil {
			for _, ferr := range fetches.Errors() {
				s.logger.Error("mqx: kafka fetch error",
					"topic", ferr.Topic,
					"partition", ferr.Partition,
					"error", ferr.Err.Error(),
				)
			}
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			// 批次中每条记录处理前检查 ctx，避免大批次阻塞取消信号
			select {
			case <-ctx.Done():
				s.commitOnExit(client, ctx)
				return
			default:
			}
			record := iter.Next()
			s.handleRecord(ctx, client, topic, group, record, handler, cfg)
		}

		// 提交本批次已标记的 offset
		if err := client.CommitMarkedOffsets(ctx); err != nil {
			if ctx.Err() != nil {
				s.commitOnExit(client, ctx)
				return
			}
			if s.logger != nil {
				s.logger.Error("mqx: kafka commit offsets failed", "error", err.Error())
			}
		}
	}
}

// consumeConcurrent 并发消费：N 个独立 goroutine，各自 PollFetches。
// franz-go 自动在 poller 间分配 partition。
func (s *Subscriber) consumeConcurrent(ctx context.Context, client *kgo.Client, topic, group string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.consumeLoop(ctx, client, topic, group, handler, cfg)
		}()
	}
	wg.Wait()
}

// handleRecord 处理单条记录：handler 返回 nil → MarkCommit，error → 重试，耗尽 → DLQ + MarkCommit。
func (s *Subscriber) handleRecord(ctx context.Context, client *kgo.Client, topic, group string, record *kgo.Record, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	msg := fromKafkaRecord(record)

	// manualAck 标记 handler 是否已通过 AckFn/NackFn 手动确认。
	// error 路径检测此标记以跳过 DLQ/Ack，避免覆盖 handler 的确认决策。
	manualAck := &mqx.ManualAck{}

	// AutoAck=false 时注入手动确认函数（Ack/Nack 由 mqx.ManualAck 保证只生效一次）
	if !cfg.AutoAck {
		manualAck.Inject(msg,
			func() error {
				client.MarkCommitRecords(record)
				return nil
			},
			func(requeue bool) error {
				if !requeue {
					client.MarkCommitRecords(record)
				}
				// requeue=true: 不标记 commit，重启/重平衡后重投
				return nil
			},
		)
	}

	err := handler(ctx, msg)
	if err == nil {
		if cfg.AutoAck && !manualAck.Manual() {
			client.MarkCommitRecords(record)
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
		// 写入 DLQ topic（best-effort，使用独立 context 避免 ctx 取消导致丢失）
		dlqRecord := toKafkaRecord(msg, map[string]string{
			"original_topic": topic,
			"original_group": group,
			"error":          err.Error(),
		})
		dlqRecord.Topic = cfg.DLQTopic
		dlqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		results := client.ProduceSync(dlqCtx, dlqRecord)
		if dlqErr := results.FirstErr(); dlqErr != nil && s.logger != nil {
			s.logger.Error("mqx: kafka DLQ write failed",
				"dlq_topic", cfg.DLQTopic,
				"original_topic", topic,
				"error", dlqErr.Error(),
			)
		}
	}

	// 标记已处理（跳过该消息，不再重试）。即使用户已手动 ack/nack，MarkCommitRecords 调用也是安全的。
	client.MarkCommitRecords(record)
}

// commitOnExit 退出前尝试提交已标记的 offset。
func (s *Subscriber) commitOnExit(client *kgo.Client, ctx context.Context) {
	commitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.CommitMarkedOffsets(commitCtx)
}

// Close 实现 mqx.Subscriber。
// Close 不会中断正在运行的 Subscribe。应取消 ctx 等待 Subscribe 返回后再调用 Close。
func (s *Subscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.client != nil {
		s.client.Close()
	}
	return nil
}

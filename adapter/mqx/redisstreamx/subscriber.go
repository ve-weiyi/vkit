package redisstreamx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Subscriber = (*Subscriber)(nil)

// Subscriber Redis Stream 消息消费器。
// 基于 Consumer Group + XREADGROUP 实现 Push 模型的消费循环。
type Subscriber struct {
	mu     sync.Mutex
	client redis.UniversalClient
	closed bool
	logger mqx.Logger

	defaultOpts []mqx.SubscribeOption
}

func newSubscriber(client redis.UniversalClient, logger mqx.Logger, opts []mqx.SubscribeOption) (*Subscriber, error) {
	return &Subscriber{
		client:      client,
		logger:      logger,
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
		return fmt.Errorf("%w: group is required for Redis Stream subscriber", mqx.ErrBadMessage)
	}

	// 确保 Consumer Group 已创建（幂等）
	if err := s.ensureConsumerGroup(ctx, topic, cfg.Group); err != nil {
		return fmt.Errorf("%w: create consumer group: %v", mqx.ErrSubscribeFailed, err)
	}

	// 若配置了 ClaimIdle，启动时 reclaim 同 group 其他 consumer 的闲置 pending 消息
	if cfg.ClaimIdle > 0 {
		count := max(cfg.Concurrency, 1)
		for i := 0; i < count; i++ {
			consumerName := cfg.Group + "-consumer"
			if count > 1 {
				consumerName = fmt.Sprintf("%s-consumer-%d", cfg.Group, i)
			}
			start := "0-0"
			for {
				claimed, nextStart, err := s.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
					Stream:   topic,
					Group:    cfg.Group,
					Consumer: consumerName,
					MinIdle:  cfg.ClaimIdle,
					Start:    start,
				}).Result()
				if err != nil {
					if s.logger != nil {
						s.logger.Error("mqx: redis XAutoClaim failed",
							"stream", topic,
							"group", cfg.Group,
							"consumer", consumerName,
							"error", err.Error(),
						)
					}
					break
				}
				for _, xMsg := range claimed {
					s.handleMessage(ctx, topic, cfg.Group, xMsg, handler, cfg)
				}
				if nextStart == "0-0" || len(claimed) == 0 {
					break
				}
				start = nextStart
			}
		}
	}

	if cfg.Concurrency <= 1 {
		consumer := cfg.Group + "-consumer"
		s.consumeLoop(ctx, topic, cfg.Group, consumer, handler, cfg)
	} else {
		s.consumeConcurrent(ctx, topic, cfg.Group, handler, cfg)
	}

	return nil
}

// ensureConsumerGroup 创建 Consumer Group（若已存在则忽略）。
func (s *Subscriber) ensureConsumerGroup(ctx context.Context, stream, group string) error {
	err := s.client.XGroupCreateMkStream(ctx, stream, group, "$").Err()
	if err != nil && strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

// consumeLoop 串行消费循环。XREADGROUP 阻塞读取 → 逐条分发给 handler。
func (s *Subscriber) consumeLoop(ctx context.Context, stream, group, consumer string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	var (
		errCount int
		backoff  = 100 * time.Millisecond
	)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    int64(cfg.Prefetch),
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				errCount = 0
				continue
			}
			if ctx.Err() != nil {
				return
			}
			// 不可恢复的错误直接退出（如 Consumer Group 被删除）
			if strings.Contains(err.Error(), "NOGROUP") {
				if s.logger != nil {
					s.logger.Error("mqx: redis consumer group lost",
						"stream", stream,
						"group", group,
						"error", err.Error(),
					)
				}
				return
			}
			if s.logger != nil {
				s.logger.Error("mqx: redis XREADGROUP failed",
					"stream", stream,
					"group", group,
					"error", err.Error(),
				)
			}
			// 指数退避，避免错误风暴
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			errCount++
			backoff = time.Duration(float64(100*time.Millisecond) * math.Pow(2, float64(errCount)))
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}

		// 成功后重置退避
		errCount = 0
		backoff = 100 * time.Millisecond

		if len(msgs) == 0 {
			continue
		}
		for _, xMsg := range msgs[0].Messages {
			s.handleMessage(ctx, stream, group, xMsg, handler, cfg)
		}
	}
}

// consumeConcurrent 并发消费：N 个独立 goroutine，各自作为 group 内的不同 consumer。
// Redis Stream Consumer Group 自动在 consumers 间负载均衡。
func (s *Subscriber) consumeConcurrent(ctx context.Context, stream, group string, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			consumer := fmt.Sprintf("%s-consumer-%d", group, n)
			s.consumeLoop(ctx, stream, group, consumer, handler, cfg)
		}(i)
	}
	wg.Wait()
}

// handleMessage 处理单条消息：handler 返回 nil → XACK，error → 重试，耗尽 → DLQ + XACK。
func (s *Subscriber) handleMessage(ctx context.Context, stream, group string, xMsg redis.XMessage, handler mqx.Handler, cfg *mqx.SubscribeConfig) {
	msg := fromRedis(xMsg.Values)

	// manualAck 标记 handler 是否已通过 AckFn/NackFn 手动确认。
	// error 路径检测此标记以跳过 DLQ/Ack，避免覆盖 handler 的确认决策。
	manualAck := &mqx.ManualAck{}

	// AutoAck=false 时注入手动确认函数（Ack/Nack 由 mqx.ManualAck 保证只生效一次）
	if !cfg.AutoAck {
		doXAck := func() {
			ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.client.XAck(ackCtx, stream, group, xMsg.ID).Err(); err != nil && s.logger != nil {
				s.logger.Error("mqx: redis XAck failed",
					"stream", stream,
					"group", group,
					"message_id", xMsg.ID,
					"error", err.Error(),
				)
			}
		}
		manualAck.Inject(msg,
			func() error {
				doXAck()
				return nil
			},
			func(requeue bool) error {
				if !requeue {
					doXAck()
				}
				// requeue=true: 不 XAck，留在 pending 等待 XAutoClaim 回收
				return nil
			},
		)
	}

	err := handler(ctx, msg)
	if err == nil {
		if cfg.AutoAck && !manualAck.Manual() {
			if ackErr := s.client.XAck(ctx, stream, group, xMsg.ID).Err(); ackErr != nil && s.logger != nil {
				s.logger.Error("mqx: redis XAck failed",
					"stream", stream,
					"group", group,
					"message_id", xMsg.ID,
					"error", ackErr.Error(),
				)
			}
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
		// 写入 DLQ Stream（best-effort，使用独立 context 避免 ctx 取消导致丢失）
		dlqFields := toRedis(msg, map[string]string{
			"original_stream": stream,
			"original_group":  group,
			"error":           err.Error(),
		})
		dlqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if dlqErr := s.client.XAdd(dlqCtx, &redis.XAddArgs{
			Stream: cfg.DLQTopic,
			Values: dlqFields,
		}).Err(); dlqErr != nil && s.logger != nil {
			s.logger.Error("mqx: redis DLQ write failed",
				"dlq_stream", cfg.DLQTopic,
				"original_stream", stream,
				"error", dlqErr.Error(),
			)
		}
	}

	// XACK 移除 pending（使用独立 context 避免 ctx 取消导致 pending 残留）
	ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ackErr := s.client.XAck(ackCtx, stream, group, xMsg.ID).Err(); ackErr != nil && s.logger != nil {
		s.logger.Error("mqx: redis XAck failed",
			"stream", stream,
			"group", group,
			"message_id", xMsg.ID,
			"error", ackErr.Error(),
		)
	}
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
	return nil
}

package redisstreamx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type publisher struct {
	broker *Broker
}

var (
	_ mqx.Publisher      = (*publisher)(nil)
	_ mqx.BatchPublisher = (*publisher)(nil)
)

// Publish 实现 mqx.Publisher。
func (p *publisher) Publish(ctx context.Context, msg *mqx.Message) (*mqx.PublishResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", mqx.ErrBadMessage)
	}
	prepared, err := msg.Prepared()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	_, err = p.broker.client.XAdd(ctx, &redis.XAddArgs{
		Stream: prepared.Destination,
		Values: toRedis(&prepared),
	}).Result()
	if metrics := p.broker.cfg.Metrics; metrics != nil {
		metrics.PublishCompleted(prepared.Destination, prepared.RoutingKey, time.Since(start), err)
	}
	if err != nil {
		return nil, ClassifyPublishError(err)
	}
	return &mqx.PublishResult{ID: prepared.ID}, nil
}

// ClassifyPublishError 区分 Redis 的协议级永久错误与网络/超时类瞬时错误；
// 永久错误用 mqx.Permanent 包装，其余返回 ErrUnavailable。
func ClassifyPublishError(err error) error {
	var redisErr redis.Error
	if errors.As(err, &redisErr) {
		message := redisErr.Error()
		for _, prefix := range []string{"NOAUTH", "NOPERM", "WRONGTYPE", "ERR ", "EXECABORT"} {
			if strings.HasPrefix(message, prefix) {
				return mqx.Permanent(fmt.Errorf("%w: %v", mqx.ErrUnavailable, err))
			}
		}
	}
	return fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
}

// PublishBatch 实现 mqx.BatchPublisher。
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

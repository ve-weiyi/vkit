package kafkax

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

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

	record := &kgo.Record{
		Topic:     prepared.Destination,
		Key:       []byte(prepared.Key),
		Value:     prepared.Body,
		Timestamp: prepared.Timestamp,
		Headers:   toHeaders(&prepared),
	}

	start := time.Now()
	results := p.broker.producer.ProduceSync(ctx, record)
	err = results.FirstErr()
	if metrics := p.broker.cfg.Metrics; metrics != nil {
		metrics.PublishCompleted(prepared.Destination, prepared.RoutingKey, time.Since(start), err)
	}
	if err != nil {
		return nil, ClassifyPublishError(err)
	}
	result := results[0]
	return &mqx.PublishResult{
		ID:        prepared.ID,
		Partition: result.Record.Partition,
		Offset:    result.Record.Offset,
	}, nil
}

// PublishBatch 实现 mqx.BatchPublisher。
func (p *publisher) PublishBatch(ctx context.Context, msgs []*mqx.Message) ([]mqx.PublishResult, error) {
	prepared := make([]mqx.Message, len(msgs))
	records := make([]*kgo.Record, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			return nil, fmt.Errorf("%w: message %d is nil", mqx.ErrBadMessage, i)
		}
		item, err := msg.Prepared()
		if err != nil {
			return nil, err
		}
		prepared[i] = item
		records[i] = &kgo.Record{
			Topic:     item.Destination,
			Key:       []byte(item.Key),
			Value:     item.Body,
			Timestamp: item.Timestamp,
			Headers:   toHeaders(&item),
		}
	}

	results := p.broker.producer.ProduceSync(ctx, records...)
	out := make([]mqx.PublishResult, len(prepared))
	var firstErr error
	for i, result := range results {
		if result.Err != nil {
			if firstErr == nil {
				firstErr = result.Err
			}
			continue
		}
		out[i] = mqx.PublishResult{
			ID:        prepared[i].ID,
			Partition: result.Record.Partition,
			Offset:    result.Record.Offset,
		}
	}
	if firstErr != nil {
		return out, ClassifyPublishError(firstErr)
	}
	return out, nil
}

// ClassifyPublishError 区分 Kafka 的永久错误（如权限、消息过大）与瞬时错误；
// 永久错误用 mqx.Permanent 包装，其余返回 ErrUnavailable。
func ClassifyPublishError(err error) error {
	var kerrErr *kerr.Error
	if errors.As(err, &kerrErr) && !kerrErr.Retriable && kerrErr.Code != kerr.UnknownServerError.Code {
		return mqx.Permanent(fmt.Errorf("%w: %s (%d)", mqx.ErrUnavailable, kerrErr.Message, kerrErr.Code))
	}
	return fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
}

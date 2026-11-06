package event

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// PublishOption 在事件默认信封之上补充投递元数据。
type PublishOption func(*publishConfig)

type publishConfig struct {
	id        string
	key       string
	headers   map[string]string
	timestamp time.Time
}

// WithID 指定消息 ID；outbox 复用同一 ID 时使用。
func WithID(id string) PublishOption {
	return func(c *publishConfig) { c.id = id }
}

// WithKey 设置分区/排序键；Kafka 用于选择 partition。
func WithKey(key string) PublishOption {
	return func(c *publishConfig) { c.key = key }
}

// WithHeaders 合并额外消息头，不覆盖事件信封字段。
func WithHeaders(headers map[string]string) PublishOption {
	return func(c *publishConfig) {
		if len(headers) == 0 {
			return
		}
		if c.headers == nil {
			c.headers = make(map[string]string, len(headers))
		}
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithTimestamp 覆盖事件时间，缺省为发布时刻。
func WithTimestamp(timestamp time.Time) PublishOption {
	return func(c *publishConfig) {
		if !timestamp.IsZero() {
			c.timestamp = timestamp
		}
	}
}

// Message 构造事件消息（含信封），但不发送。
//
// outbox 与直接发布共用同一条构造路径，避免信封字段分叉。
func (e EventSpec[T]) Message(event *T, opts ...PublishOption) (mqx.Message, error) {
	if e.destination == "" {
		return mqx.Message{}, fmt.Errorf("%w: destination is required", mqx.ErrBadMessage)
	}
	if e.routingKey == "" {
		return mqx.Message{}, fmt.Errorf("%w: routing key is required", mqx.ErrBadMessage)
	}

	body, err := e.codec.Marshal(event)
	if err != nil {
		return mqx.Message{}, fmt.Errorf("event: marshal %s: %w", e.routingKey, err)
	}

	cfg := publishConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.id == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return mqx.Message{}, fmt.Errorf("event: generate id: %w", err)
		}
		cfg.id = id.String()
	}
	if cfg.timestamp.IsZero() {
		cfg.timestamp = time.Now()
	}

	headers := make(map[string]string, len(cfg.headers)+5)
	for k, v := range cfg.headers {
		headers[k] = v
	}
	headers[mqx.HeaderEventID] = cfg.id
	headers[mqx.HeaderEventType] = e.routingKey
	headers[mqx.HeaderOccurredAt] = cfg.timestamp.Format(time.RFC3339Nano)
	if e.version != "" {
		headers[mqx.HeaderSchemaVersion] = e.version
	}
	if e.producer != "" {
		headers[mqx.HeaderProducer] = e.producer
	}

	return mqx.Message{
		ID:          cfg.id,
		Destination: e.destination,
		RoutingKey:  e.routingKey,
		Key:         cfg.key,
		Headers:     headers,
		Body:        body,
		Timestamp:   cfg.timestamp,
	}, nil
}

// Publish 序列化事件、写入信封并投递。
//
// 发布器不可用时返回 ErrUnavailable，便于调用方按普通错误处理。
func (e EventSpec[T]) Publish(ctx context.Context, pub mqx.Publisher, event *T, opts ...PublishOption) (*mqx.PublishResult, error) {
	if pub == nil {
		return nil, mqx.ErrUnavailable
	}
	msg, err := e.Message(event, opts...)
	if err != nil {
		return nil, err
	}
	return pub.Publish(ctx, &msg)
}

package redisstreamx

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Publisher = (*Publisher)(nil)

// Publisher Redis Stream 消息发布器。通过 XADD 写入 Stream。
type Publisher struct {
	mu     sync.Mutex
	client redis.UniversalClient
	closed bool

	defaultOpts []mqx.PublishOption
}

func newPublisher(client redis.UniversalClient, opts []mqx.PublishOption) (*Publisher, error) {
	return &Publisher{client: client, defaultOpts: opts}, nil
}

// Publish 实现 mqx.Publisher。使用 XADD 将消息写入 Stream。
func (p *Publisher) Publish(ctx context.Context, topic string, msg *mqx.Message, opts ...mqx.PublishOption) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return mqx.ErrClosed
	}
	client := p.client
	p.mu.Unlock()

	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Delay > 0 {
		return fmt.Errorf("%w: delayed publish is not supported with Redis Stream", mqx.ErrNotSupported)
	}

	fields := toRedis(msg, cfg.Headers)
	if cfg.Key != "" {
		fields[fieldKey] = cfg.Key
	}

	publishCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	if _, err := client.XAdd(publishCtx, &redis.XAddArgs{
		Stream: topic,
		Values: fields,
	}).Result(); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrSendFailed, err)
	}

	return nil
}

// PublishBatch 实现 mqx.Publisher。使用 Pipeline 批量写入以减少 RTT。
func (p *Publisher) PublishBatch(ctx context.Context, topic string, msgs []*mqx.Message, opts ...mqx.PublishOption) error {
	if len(msgs) == 0 {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return mqx.ErrClosed
	}
	client := p.client
	p.mu.Unlock()

	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Delay > 0 {
		return fmt.Errorf("%w: delayed publish is not supported with Redis Stream", mqx.ErrNotSupported)
	}

	publishCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	pipe := client.Pipeline()
	for _, msg := range msgs {
		fields := toRedis(msg, cfg.Headers)
		if cfg.Key != "" {
			fields[fieldKey] = cfg.Key
		}
		pipe.XAdd(publishCtx, &redis.XAddArgs{
			Stream: topic,
			Values: fields,
		})
	}

	cmders, err := pipe.Exec(publishCtx)
	if err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrSendFailed, err)
	}
	for i, cmd := range cmders {
		if cmd.Err() != nil {
			return fmt.Errorf("message %d/%d: %w: %v", i+1, len(msgs), mqx.ErrSendFailed, cmd.Err())
		}
	}

	return nil
}

// Close 实现 mqx.Publisher。
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}

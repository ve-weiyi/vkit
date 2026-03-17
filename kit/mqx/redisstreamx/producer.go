package redisstreamx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// producer Redis Stream生产者实现
type producer struct {
	client *redis.Client
	config *Config
}

// newProducer 创建生产者
func newProducer(client *redis.Client, config *Config) (*producer, error) {
	return &producer{
		client: client,
		config: config,
	}, nil
}

// Send 发送单条消息
func (p *producer) Send(ctx context.Context, message *mqx.Message) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// 构建Redis Stream消息
	values := map[string]interface{}{
		"id":        message.ID,
		"key":       message.Key,
		"body":      message.Body,
		"timestamp": message.Timestamp.Unix(),
	}

	// 添加Headers
	if message.Headers != nil {
		headersJSON, err := json.Marshal(message.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
		values["headers"] = headersJSON
	}

	// 发送到Redis Stream
	_, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: message.Topic,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendBatch 批量发送消息
func (p *producer) SendBatch(ctx context.Context, messages []*mqx.Message) error {
	// Redis Stream不支持原生批量发送，使用Pipeline
	pipe := p.client.Pipeline()

	for _, message := range messages {
		values := map[string]interface{}{
			"id":        message.ID,
			"key":       message.Key,
			"body":      message.Body,
			"timestamp": message.Timestamp.Unix(),
		}

		if message.Headers != nil {
			headersJSON, err := json.Marshal(message.Headers)
			if err != nil {
				return fmt.Errorf("failed to marshal headers: %w", err)
			}
			values["headers"] = headersJSON
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: message.Topic,
			Values: values,
		})
	}

	// 执行Pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to send batch messages: %w", err)
	}

	return nil
}

// Close 关闭生产者
func (p *producer) Close() error {
	// Redis Stream生产者不需要关闭
	return nil
}

// 确保实现了接口
var _ mqx.Producer = (*producer)(nil)

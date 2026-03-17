package redisstreamx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// consumer Redis Stream消费者实现
type consumer struct {
	client         *redis.Client
	redisConfig    *Config
	consumerConfig *mqx.ConsumerConfig

	topics  []string
	closeCh chan struct{}
	closed  bool
	mu      sync.RWMutex

	logger logx.Logger
}

// newConsumer 创建消费者
func newConsumer(client *redis.Client, redisConfig *Config, consumerConfig *mqx.ConsumerConfig) (*consumer, error) {
	if consumerConfig.GroupID == "" {
		return nil, fmt.Errorf("GroupID is required for Redis Stream consumer")
	}

	return &consumer{
		client:         client,
		redisConfig:    redisConfig,
		consumerConfig: consumerConfig,
		closeCh:        make(chan struct{}),
		logger:         logx.WithContext(context.Background()),
	}, nil
}

// Start 启动消费者
func (c *consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	return nil
}

// Stop 停止消费者（优雅关闭）
func (c *consumer) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.closeCh)

	return nil
}

// Subscribe 订阅主题/队列
func (c *consumer) Subscribe(topics ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(topics) == 0 {
		return fmt.Errorf("topics cannot be empty")
	}

	c.topics = topics

	// 为每个stream创建消费者组
	ctx := context.Background()
	for _, topic := range topics {
		// 尝试创建消费者组（如果已存在会报错，忽略）
		err := c.client.XGroupCreateMkStream(ctx, topic, c.consumerConfig.GroupID, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			c.logger.Errorf("failed to create consumer group for topic %s: %v", topic, err)
		}
	}

	return nil
}

// Consume 消费消息（返回消息通道）
func (c *consumer) Consume() (<-chan *mqx.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.topics) == 0 {
		return nil, fmt.Errorf("not subscribed to any topic, call Subscribe first")
	}

	messageCh := make(chan *mqx.Message, 100)

	// 启动消费循环
	go func() {
		defer close(messageCh)

		ctx := context.Background()
		consumerName := c.consumerConfig.ConsumerName
		if consumerName == "" {
			consumerName = "consumer-1"
		}

		for {
			select {
			case <-c.closeCh:
				return
			default:
				// 从每个stream读取消息
				for _, topic := range c.topics {
					// 读取消息
					streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
						Group:    c.consumerConfig.GroupID,
						Consumer: consumerName,
						Streams:  []string{topic, ">"},
						Count:    int64(c.consumerConfig.PrefetchCount),
						Block:    time.Second,
					}).Result()

					if err != nil {
						if err != redis.Nil {
							c.logger.Errorf("failed to read from stream %s: %v", topic, err)
						}
						continue
					}

					// 处理消息
					for _, stream := range streams {
						for _, message := range stream.Messages {
							mqxMessage := c.convertToMessage(topic, message)
							messageCh <- mqxMessage
						}
					}
				}
			}
		}
	}()

	return messageCh, nil
}

// ConsumeWithHandler 使用处理器消费（自动处理 Ack/Nack）
func (c *consumer) ConsumeWithHandler(handler mqx.MessageHandler) error {
	messageCh, err := c.Consume()
	if err != nil {
		return err
	}

	// 启动消费循环
	go func() {
		for message := range messageCh {
			ctx := context.Background()

			// 处理消息
			err := handler.Handle(ctx, message)
			if err != nil {
				c.logger.Errorf("failed to handle message: %v, message: %s", err, string(message.Body))
				// 处理失败，不确认消息
				message.Nack(true)
			} else {
				// 处理成功，确认消息
				message.Ack()
			}
		}
	}()

	return nil
}

// Close 关闭消费者
func (c *consumer) Close() error {
	c.Stop(context.Background())
	return nil
}

// convertToMessage 转换Redis Stream消息为统一消息格式
func (c *consumer) convertToMessage(topic string, message redis.XMessage) *mqx.Message {
	mqxMessage := &mqx.Message{
		ID:      message.ID,
		Topic:   topic,
		Headers: make(map[string]interface{}),
	}

	// 设置原始消息
	mqxMessage.SetRaw(message)

	// 解析消息字段
	if id, ok := message.Values["id"].(string); ok {
		mqxMessage.ID = id
	}
	if key, ok := message.Values["key"].(string); ok {
		mqxMessage.Key = key
	}
	if body, ok := message.Values["body"].(string); ok {
		mqxMessage.Body = []byte(body)
	}
	if timestamp, ok := message.Values["timestamp"].(string); ok {
		var ts int64
		fmt.Sscanf(timestamp, "%d", &ts)
		mqxMessage.Timestamp = time.Unix(ts, 0)
	}

	// 解析Headers
	if headersJSON, ok := message.Values["headers"].(string); ok {
		json.Unmarshal([]byte(headersJSON), &mqxMessage.Headers)
	}

	// 设置Ack/Nack函数
	ctx := context.Background()
	mqxMessage.SetAckFunc(func() error {
		// 确认消息（从pending列表中删除）
		return c.client.XAck(ctx, topic, c.consumerConfig.GroupID, message.ID).Err()
	})
	mqxMessage.SetNackFunc(func(requeue bool) error {
		// Redis Stream没有Nack概念，这里不做任何操作
		// 消息会保留在pending列表中，可以被重新消费
		return nil
	})

	return mqxMessage
}

// 确保实现了接口
var _ mqx.Consumer = (*consumer)(nil)

package kafkax

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// consumer Kafka消费者实现
type consumer struct {
	client         sarama.Client
	consumerGroup  sarama.ConsumerGroup
	kafkaConfig    *Config
	consumerConfig *mqx.ConsumerConfig

	handler *consumerGroupHandler
	closeCh chan struct{}
	closed  bool
	mu      sync.RWMutex

	logger logx.Logger
}

// newConsumer 创建消费者
func newConsumer(client sarama.Client, kafkaConfig *Config, consumerConfig *mqx.ConsumerConfig) (*consumer, error) {
	if consumerConfig.GroupID == "" {
		return nil, fmt.Errorf("GroupID is required for Kafka consumer")
	}

	// 创建Sarama配置
	config := sarama.NewConfig()
	config.Version = client.Config().Version

	// 配置消费者
	config.Consumer.Return.Errors = true

	// 配置偏移量
	switch consumerConfig.StartOffset {
	case mqx.OffsetEarliest:
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	case mqx.OffsetLatest:
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
	default:
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	// 自动提交配置
	if consumerConfig.AutoCommit {
		config.Consumer.Offsets.AutoCommit.Enable = true
		if consumerConfig.CommitInterval > 0 {
			config.Consumer.Offsets.AutoCommit.Interval = consumerConfig.CommitInterval
		}
	} else {
		config.Consumer.Offsets.AutoCommit.Enable = false
	}

	// 创建消费者组
	consumerGroup, err := sarama.NewConsumerGroupFromClient(consumerConfig.GroupID, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	c := &consumer{
		client:         client,
		consumerGroup:  consumerGroup,
		kafkaConfig:    kafkaConfig,
		consumerConfig: consumerConfig,
		closeCh:        make(chan struct{}),
		logger:         logx.WithContext(context.Background()),
	}

	c.handler = &consumerGroupHandler{
		consumer: c,
		ready:    make(chan bool),
	}

	return c, nil
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

	c.consumerConfig.Topics = topics

	// 启动消费循环
	go func() {
		for {
			select {
			case <-c.closeCh:
				return
			default:
				// 消费消息
				err := c.consumerGroup.Consume(context.Background(), topics, c.handler)
				if err != nil {
					c.logger.Errorf("consumer group error: %v", err)
				}
			}
		}
	}()

	// 等待消费者准备就绪
	<-c.handler.ready

	return nil
}

// Consume 消费消息（返回消息通道）
func (c *consumer) Consume() (<-chan *mqx.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.consumerConfig.Topics) == 0 {
		return nil, fmt.Errorf("not subscribed to any topic, call Subscribe first")
	}

	messageCh := make(chan *mqx.Message, 100)
	c.handler.messageCh = messageCh

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
				// Kafka的Nack会跳过该消息，不会重新入队
				message.Nack(false)
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

	if c.consumerGroup != nil {
		return c.consumerGroup.Close()
	}
	return nil
}

// 确保实现了接口
var _ mqx.Consumer = (*consumer)(nil)

// consumerGroupHandler Kafka消费者组处理器
type consumerGroupHandler struct {
	consumer  *consumer
	messageCh chan *mqx.Message
	ready     chan bool
	once      sync.Once
}

// Setup 在消费者组会话开始时调用
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	// 标记消费者准备就绪
	h.once.Do(func() {
		close(h.ready)
	})
	return nil
}

// Cleanup 在消费者组会话结束时调用
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 处理消息
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case message := <-claim.Messages():
			if message == nil {
				continue
			}

			// 转换为统一消息格式
			mqxMessage := h.convertToMessage(message, session)

			// 发送到消息通道
			if h.messageCh != nil {
				h.messageCh <- mqxMessage
			}
		}
	}
}

// convertToMessage 转换Kafka消息为统一消息格式
func (h *consumerGroupHandler) convertToMessage(message *sarama.ConsumerMessage, session sarama.ConsumerGroupSession) *mqx.Message {
	mqxMessage := &mqx.Message{
		ID:        fmt.Sprintf("%s-%d-%d", message.Topic, message.Partition, message.Offset),
		Topic:     message.Topic,
		Key:       string(message.Key),
		Body:      message.Value,
		Headers:   make(map[string]interface{}),
		Timestamp: message.Timestamp,
		Partition: message.Partition,
		Offset:    message.Offset,
	}

	// 设置原始消息
	mqxMessage.SetRaw(message)

	// 转换Headers
	if message.Headers != nil {
		for _, header := range message.Headers {
			mqxMessage.Headers[string(header.Key)] = string(header.Value)
		}
	}

	// 设置Ack/Nack函数
	mqxMessage.SetAckFunc(func() error {
		session.MarkMessage(message, "")
		return nil
	})
	mqxMessage.SetNackFunc(func(requeue bool) error {
		// Kafka没有Nack概念，这里只是跳过该消息
		// 如果需要重试，可以发送到重试topic
		return nil
	})

	return mqxMessage
}

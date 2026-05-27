package rabbitmqx

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// consumer RabbitMQ消费者实现
type consumer struct {
	mq     *RabbitMQ
	config *Config

	channel        *amqp.Channel
	consumerConfig *mqx.ConsumerConfig

	deliveries <-chan amqp.Delivery
	closeCh    chan struct{}
	closed     bool
	mu         sync.RWMutex

	logger logx.Logger
}

// newConsumer 创建消费者
func newConsumer(mq *RabbitMQ, rabbitConfig *Config, consumerConfig *mqx.ConsumerConfig) (*consumer, error) {
	conn := mq.Conn()
	if conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("connection is closed")
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// 声明交换机
	err = channel.ExchangeDeclare(
		rabbitConfig.ExchangeName,
		rabbitConfig.ExchangeType,
		rabbitConfig.Durable,
		rabbitConfig.AutoDelete,
		false,
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// 设置QoS（流控）
	if consumerConfig.PrefetchCount > 0 {
		err = channel.Qos(
			consumerConfig.PrefetchCount, // prefetch count
			0,                            // prefetch size
			false,                        // global
		)
		if err != nil {
			channel.Close()
			return nil, fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	return &consumer{
		mq:             mq,
		channel:        channel,
		config:         rabbitConfig,
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

	// 确定队列名称和配置
	var queueName string
	var queueDurable, queueAutoDelete, queueExclusive bool
	var queueArgs amqp.Table

	if c.config.QueueConfig != nil {
		// 使用自定义队列配置
		queueName = c.config.QueueConfig.QueueName
		queueDurable = c.config.QueueConfig.Durable
		queueAutoDelete = c.config.QueueConfig.AutoDelete
		queueExclusive = c.config.QueueConfig.Exclusive
		if c.config.QueueConfig.Args != nil {
			queueArgs = amqp.Table(c.config.QueueConfig.Args)
		}
	} else {
		// 使用默认配置：队列名称取第一个topic，继承交换机的持久化和自动删除配置
		queueName = topics[0]
		queueDurable = c.config.Durable
		queueAutoDelete = c.config.AutoDelete
		queueExclusive = false
	}

	// 如果队列名称为空，使用第一个topic作为队列名
	if queueName == "" {
		queueName = topics[0]
	}

	// 声明队列
	queue, err := c.channel.QueueDeclare(
		queueName,       // 队列名称
		queueDurable,    // 是否持久化
		queueAutoDelete, // 是否自动删除
		queueExclusive,  // exclusive
		false,           // no-wait
		queueArgs,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// 绑定队列到交换机
	// 如果有多个topics，将它们作为路由键绑定
	for _, topic := range topics {
		err = c.channel.QueueBind(
			queue.Name,            // 队列名称
			topic,                 // 路由键
			c.config.ExchangeName, // 交换机名称
			false,                 // no-wait
			nil,                   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}
	}

	// 开始消费
	deliveries, err := c.channel.Consume(
		queue.Name,                    // 队列名称
		c.consumerConfig.ConsumerName, // 消费者标签
		false,                         // auto-ack (手动确认)
		queueExclusive,                // exclusive
		false,                         // no-local
		false,                         // no-wait
		nil,                           // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to consume: %w", err)
	}

	c.deliveries = deliveries

	return nil
}

// Consume 消费消息（返回消息通道）
func (c *consumer) Consume() (<-chan *mqx.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.deliveries == nil {
		return nil, fmt.Errorf("not subscribed to any topic, call Subscribe first")
	}

	messageCh := make(chan *mqx.Message, 100)

	go func() {
		defer close(messageCh)

		for {
			select {
			case <-c.closeCh:
				return
			case delivery, ok := <-c.deliveries:
				if !ok {
					return
				}

				// 转换为统一消息格式
				message := c.convertToMessage(delivery)
				messageCh <- message
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
				c.logger.Errorf("failed to handle message: %v, id=%s, body=%s", err, message.ID, string(message.Body))

				// 判断是否需要重试
				message.Attempts++
				maxRetries := c.consumerConfig.MaxRetries
				if maxRetries <= 0 {
					maxRetries = 3 // 默认最多重试3次
				}

				if message.Attempts <= maxRetries {
					// 未达到最大重试次数，重新入队
					c.logger.Infof("message %s retry attempt=%d/%d, requeue", message.ID, message.Attempts, maxRetries)
					message.Nack(true)
				} else {
					// 超过最大重试次数，不重新入队，发送到死信队列
					c.logger.Errorf("message %s exceeded max retries (%d), sending to DLQ", message.ID, maxRetries)
					message.Nack(false)

					// 如果配置了死信队列，手动发送到DLQ
					if c.consumerConfig.EnableDLQ && c.consumerConfig.DLQTopic != "" {
						c.sendToDLQ(ctx, message)
					}
				}
			} else {
				// 处理成功，确认消息
				message.Ack()
			}
		}
	}()

	return nil
}

// sendToDLQ 发送消息到死信队列
func (c *consumer) sendToDLQ(ctx context.Context, message *mqx.Message) {
	// 通过 RabbitMQ 实例创建临时生产者发送到死信队列
	dlqMsg := &mqx.Message{
		ID:        fmt.Sprintf("%s-dlq", message.ID),
		Topic:     c.consumerConfig.DLQTopic,
		Key:       c.consumerConfig.DLQTopic,
		Body:      message.Body,
		Headers:   message.Headers,
		Timestamp: message.Timestamp,
	}
	if dlqMsg.Headers == nil {
		dlqMsg.Headers = make(map[string]interface{})
	}
	dlqMsg.Headers["x-original-topic"] = message.Topic
	dlqMsg.Headers["x-original-id"] = message.ID
	dlqMsg.Headers["x-retry-attempts"] = message.Attempts

	// 使用同一个RabbitMQ连接发送DLQ消息
	dlqProducer, err := newProducer(c.mq, c.config)
	if err != nil {
		c.logger.Errorf("failed to create DLQ producer: %v", err)
		return
	}
	defer dlqProducer.Close()

	if err := dlqProducer.Send(ctx, dlqMsg); err != nil {
		c.logger.Errorf("failed to send message to DLQ: %v", err)
	} else {
		c.logger.Infof("message %s sent to DLQ: %s", message.ID, c.consumerConfig.DLQTopic)
	}
}

// Close 关闭消费者
func (c *consumer) Close() error {
	c.Stop(context.Background())

	if c.channel != nil {
		return c.channel.Close()
	}
	return nil
}

// convertToMessage 转换AMQP消息为统一消息格式
func (c *consumer) convertToMessage(delivery amqp.Delivery) *mqx.Message {
	message := &mqx.Message{
		ID:        delivery.MessageId,
		Topic:     delivery.RoutingKey,
		Key:       delivery.RoutingKey,
		Body:      delivery.Body,
		Headers:   make(map[string]interface{}),
		Timestamp: delivery.Timestamp,
	}

	// 设置原始消息
	message.SetRaw(delivery)

	// 转换Headers
	if delivery.Headers != nil {
		for k, v := range delivery.Headers {
			message.Headers[k] = v
		}
	}

	// 设置Ack/Nack函数
	message.SetAckFunc(func() error {
		return delivery.Ack(false)
	})
	message.SetNackFunc(func(requeue bool) error {
		return delivery.Nack(false, requeue)
	})

	return message
}

// 确保实现了接口
var _ mqx.Consumer = (*consumer)(nil)

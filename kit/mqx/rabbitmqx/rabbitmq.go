package rabbitmqx

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// RabbitMQ RabbitMQ实现
type RabbitMQ struct {
	conn   *amqp.Connection
	config *Config
}

// Config RabbitMQ配置
type Config struct {
	URL          string // AMQP连接URL
	ExchangeName string // 交换机名称
	ExchangeType string // 交换机类型: direct, topic, fanout, headers
	Durable      bool   // 是否持久化
	AutoDelete   bool   // 是否自动删除

	// 队列配置（可选，如果不设置则使用 topic 作为队列名）
	QueueConfig *QueueConfig
}

// QueueConfig 队列配置
type QueueConfig struct {
	QueueName  string                 // 队列名称（如果为空，使用 topic 作为队列名）
	Durable    bool                   // 队列是否持久化
	AutoDelete bool                   // 队列是否自动删除
	Exclusive  bool                   // 队列是否独占
	Args       map[string]interface{} // 队列额外参数（如：x-message-ttl, x-max-length等）
}

// NewRabbitMQ 创建RabbitMQ实例
func NewRabbitMQ(config *Config) (*RabbitMQ, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	conn, err := amqp.Dial(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return &RabbitMQ{
		conn:   conn,
		config: config,
	}, nil
}

// Producer 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Producer() (mqx.Producer, error) {
	return newProducer(r.conn, r.config)
}

// Consumer 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Consumer(config *mqx.ConsumerConfig) (mqx.Consumer, error) {
	return newConsumer(r.conn, r.config, config)
}

// Close 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Close() error {
	if r.conn != nil && !r.conn.IsClosed() {
		return r.conn.Close()
	}
	return nil
}

// Ping 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Ping(ctx context.Context) error {
	if r.conn == nil || r.conn.IsClosed() {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

// 确保实现了接口
var _ mqx.MessageQueue = (*RabbitMQ)(nil)

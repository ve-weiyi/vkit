package rabbitmqx

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// RabbitMQ RabbitMQ实现
type RabbitMQ struct {
	conn   *amqp.Connection
	config *Config

	reconnectDelay time.Duration
	reconnectOnce  sync.Once
	mu             sync.RWMutex
	closeCh        chan struct{}
	closed         bool

	logger logx.Logger
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

// ExchangeType 交换机类型
type ExchangeType string

const (
	// ExchangeTypeFanout Publish/Subscribe模式（发布/订阅模式）
	// 消息被路由投递给多个队列，一个消息被多个消费者获取
	// 发布者只需要声明交换机，不需要关心队列
	// 订阅者需要声明自己的队列并绑定到交换机
	ExchangeTypeFanout = "fanout"

	// ExchangeTypeDirect Direct模式（路由模式）
	// 消息被路由投递给符合路由规则的队列，一个消息被一个消费者获取
	// 发布者需要声明交换机和routing key
	// 订阅者需要声明交换机、队列和匹配的routing key
	ExchangeTypeDirect = "direct"

	// ExchangeTypeTopic Topic模式（主题模式）
	// 消息被路由投递给符合通配符匹配的队列，一个消息被一个消费者获取
	// 发布者需要声明交换机和routing key
	// 订阅者需要声明交换机、队列和匹配的通配符routing key
	// 其中 "*" 用于匹配一个单词，"#" 用于匹配多个单词（可以是零个）
	// 例如：kuteng.* 匹配 kuteng.hello，kuteng.# 匹配 kuteng.hello.one
	ExchangeTypeTopic = "topic"
)

// NewRabbitMQ 创建RabbitMQ实例
func NewRabbitMQ(config *Config) (*RabbitMQ, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	r := &RabbitMQ{
		config:         config,
		reconnectDelay: 5 * time.Second,
		closeCh:        make(chan struct{}),
		logger:         logx.WithContext(context.Background()),
	}

	if err := r.connect(); err != nil {
		return nil, err
	}

	r.reconnectOnce.Do(func() {
		go r.watchAndReconnect()
	})

	return r, nil
}

func (r *RabbitMQ) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, err := amqp.Dial(r.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	r.conn = conn
	return nil
}

func (r *RabbitMQ) watchAndReconnect() {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Errorf("RabbitMQ watchAndReconnect panic: %v\n%s", err, debug.Stack())
		}
	}()

	for {
		r.mu.RLock()
		if r.conn == nil {
			r.mu.RUnlock()
			return
		}
		connCloseNotify := r.conn.NotifyClose(make(chan *amqp.Error, 1))
		r.mu.RUnlock()

		select {
		case err := <-connCloseNotify:
			if err != nil {
				r.logger.Errorf("RabbitMQ connection closed: %v, reconnecting...", err)
				r.tryReconnect()
			}
		case <-r.closeCh:
			return
		}
	}
}

func (r *RabbitMQ) tryReconnect() {
	for {
		r.mu.RLock()
		closed := r.closed
		r.mu.RUnlock()
		if closed {
			return
		}

		if err := r.connect(); err != nil {
			r.logger.Errorf("RabbitMQ reconnect failed: %v, retry in %v", err, r.reconnectDelay)
			time.Sleep(r.reconnectDelay)
			continue
		}
		r.logger.Infof("RabbitMQ reconnected successfully")
		return
	}
}

// Conn 返回当前连接（线程安全）
func (r *RabbitMQ) Conn() *amqp.Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn
}

// Producer 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Producer() (mqx.Producer, error) {
	return newProducer(r, r.config)
}

// Consumer 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Consumer(config *mqx.ConsumerConfig) (mqx.Consumer, error) {
	return newConsumer(r, r.config, config)
}

// Close 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	close(r.closeCh)

	if r.conn != nil && !r.conn.IsClosed() {
		return r.conn.Close()
	}
	return nil
}

// Ping 实现 mqx.MessageQueue 接口
func (r *RabbitMQ) Ping(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.conn == nil || r.conn.IsClosed() {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

// 确保实现了接口
var _ mqx.MessageQueue = (*RabbitMQ)(nil)

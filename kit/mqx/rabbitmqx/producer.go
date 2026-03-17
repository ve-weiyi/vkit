package rabbitmqx

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// producer RabbitMQ生产者实现，持有长连接 channel，支持自动重连
type producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config

	reconnectDelay time.Duration
	reconnectOnce  sync.Once
	mu             sync.RWMutex
	closeCh        chan struct{}
	closed         bool

	logger logx.Logger
}

// newProducer 创建生产者（长连接，不每次新建）
func newProducer(conn *amqp.Connection, config *Config) (*producer, error) {
	p := &producer{
		conn:           conn,
		config:         config,
		reconnectDelay: 5 * time.Second,
		closeCh:        make(chan struct{}),
		logger:         logx.WithContext(context.Background()),
	}

	if err := p.openChannel(); err != nil {
		return nil, err
	}

	p.reconnectOnce.Do(func() {
		go p.watchAndReconnect()
	})

	return p, nil
}

func (p *producer) openChannel() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if err = ch.ExchangeDeclare(
		p.config.ExchangeName, // 交换机名称
		p.config.ExchangeType, // 交换机类型
		p.config.Durable,      // 是否持久化
		p.config.AutoDelete,   // 是否自动删除
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	); err != nil {
		ch.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	p.channel = ch
	return nil
}

func (p *producer) watchAndReconnect() {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("producer watchAndReconnect panic: %v\n%s", r, debug.Stack())
		}
	}()

	for {
		p.mu.RLock()
		chanClose := p.channel.NotifyClose(make(chan *amqp.Error, 1))
		p.mu.RUnlock()

		select {
		case err := <-chanClose:
			if err != nil {
				p.logger.Errorf("producer channel closed: %v, reconnecting...", err)
				p.tryReopen()
			}
		case <-p.closeCh:
			return
		}
	}
}

func (p *producer) tryReopen() {
	for {
		p.mu.RLock()
		closed := p.closed
		p.mu.RUnlock()
		if closed {
			return
		}

		if err := p.openChannel(); err != nil {
			p.logger.Errorf("producer reopen channel failed: %v, retry in %v", err, p.reconnectDelay)
			time.Sleep(p.reconnectDelay)
			continue
		}
		p.logger.Infof("producer channel reopened")
		return
	}
}

// Send 发送单条消息
func (p *producer) Send(ctx context.Context, message *mqx.Message) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// 构建AMQP消息
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         message.Body,
		Timestamp:    message.Timestamp,
		MessageId:    message.ID,
		DeliveryMode: amqp.Persistent, // 持久化消息
	}

	// 转换Headers
	if message.Headers != nil {
		publishing.Headers = amqp.Table(message.Headers)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 发布消息
	err := p.channel.PublishWithContext(
		ctx,
		p.config.ExchangeName, // 交换机
		message.Key,           // 路由键
		false,                 // mandatory
		false,                 // immediate
		publishing,
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// SendBatch 批量发送消息
func (p *producer) SendBatch(ctx context.Context, messages []*mqx.Message) error {
	for _, message := range messages {
		if err := p.Send(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭生产者
func (p *producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	close(p.closeCh)

	if p.channel != nil {
		return p.channel.Close()
	}
	return nil
}

// 确保实现了接口
var _ mqx.Producer = (*producer)(nil)

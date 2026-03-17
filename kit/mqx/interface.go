package mqx

import (
	"context"
)

// MessageQueue 消息队列接口
type MessageQueue interface {
	// Producer 获取生产者
	Producer() (Producer, error)

	// Consumer 创建消费者
	Consumer(config *ConsumerConfig) (Consumer, error)

	// Close 关闭连接
	Close() error

	// Ping 健康检查
	Ping(ctx context.Context) error
}

// Producer 生产者接口
type Producer interface {
	// Send 发送单条消息
	Send(ctx context.Context, message *Message) error

	// SendBatch 批量发送消息
	SendBatch(ctx context.Context, messages []*Message) error

	// Close 关闭生产者
	Close() error
}

// Consumer 消费者接口
type Consumer interface {
	// Start 启动消费者
	Start(ctx context.Context) error

	// Stop 停止消费者（优雅关闭）
	Stop(ctx context.Context) error

	// Subscribe 订阅主题/队列
	Subscribe(topics ...string) error

	// Consume 消费消息（返回消息通道）
	Consume() (<-chan *Message, error)

	// ConsumeWithHandler 使用处理器消费（自动处理 Ack/Nack）
	ConsumeWithHandler(handler MessageHandler) error

	// Close 关闭消费者
	Close() error
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	// Handle 处理消息
	// 返回 nil: 消息处理成功，自动 Ack
	// 返回 error: 消息处理失败，根据配置决定是否重试
	Handle(ctx context.Context, message *Message) error
}

// MessageHandlerFunc 函数式处理器
type MessageHandlerFunc func(ctx context.Context, message *Message) error

// Handle 实现 MessageHandler 接口
func (f MessageHandlerFunc) Handle(ctx context.Context, message *Message) error {
	return f(ctx, message)
}

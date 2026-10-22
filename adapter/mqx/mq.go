package mqx

import (
	"context"
)

// MessageQueue 持有底层连接，Publisher / Subscriber 从它派生，共享连接。
type MessageQueue interface {
	Publisher(opts ...PublishOption) (Publisher, error)
	Subscriber(opts ...SubscribeOption) (Subscriber, error)
	Ping(ctx context.Context) error
	Close() error
}

// Publisher 发布消息。
type Publisher interface {
	Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error
	PublishBatch(ctx context.Context, topic string, msgs []*Message, opts ...PublishOption) error
	Close() error
}

// Subscriber 订阅消息。Subscribe 阻塞运行直到 ctx 被取消。
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler Handler, opts ...SubscribeOption) error
	Close() error
}

// Handler 处理收到的消息。
// 返回 nil = ACK，返回 error = NACK（触发重试/DLQ）。
//
// 当 SubscribeConfig.AutoAck=false 时，handler 可调用 msg.Ack() / msg.Nack()
// 手动控制确认时机（返回值仅影响 retry/DLQ 逻辑，不再自动触发 ACK）。
type Handler func(ctx context.Context, msg *Message) error

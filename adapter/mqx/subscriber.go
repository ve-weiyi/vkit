package mqx

import "context"

// Subscriber 创建订阅。
type Subscriber interface {
	Subscribe(ctx context.Context, spec SubscriptionSpec, handler Handler) (Subscription, error)
}

// Subscription 是一个运行中的订阅。
type Subscription interface {
	// Err 只发送致命错误；缓冲 1，订阅结束后关闭。
	Err() <-chan error
	// Close 幂等：停止投递新消息，等待在途 handler 退出（受 ctx 截止时间约束），
	// 未确认的消息按后端语义重投。
	Close(ctx context.Context) error
}

// Handler 处理一条投递。
//
// 返回 nil 表示成功；返回 error 表示失败，由 RetryPolicy 决定是否重试。
// 返回 Permanent 包装的错误表示不重试；返回 RetryAfter 包装的错误表示按指定延迟重试。
// handler 也可通过 Delivery.Ack/Nack 手动确认，手动确认优先于返回值。
type Handler func(ctx context.Context, d *Delivery) error

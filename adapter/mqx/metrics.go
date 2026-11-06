package mqx

import "time"

// Metrics 是发布与消费侧指标接口，由调用方接入 Prometheus、statsd 等。
//
// 实现必须并发安全，且快速返回：它在 handler 路径上被同步调用。
type Metrics interface {
	// PublishCompleted 记录一次发布；err 非 nil 表示消息未被接受。
	PublishCompleted(destination, routingKey string, dur time.Duration, err error)
	// ConsumeCompleted 记录每次 handler 执行（含进程内重试的每次尝试，attempt 为尝试序号）。
	ConsumeCompleted(destination, routingKey, group string, attempt int, dur time.Duration, err error)
	// SubscriptionStopped 记录订阅结束；err 非 nil 表示非正常退出。
	SubscriptionStopped(destination, group string, err error)
}

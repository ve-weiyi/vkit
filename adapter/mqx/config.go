package mqx

import "time"

// ── PublishConfig & PublishOption ──

// PublishConfig 单次发布的可选配置。
type PublishConfig struct {
	Key     string
	Headers map[string]string
	Delay   time.Duration
	Timeout time.Duration
	// Extras 后端专用扩展字段。Key 建议使用 "kafkax:xxx" 等带前缀的格式以避免冲突。
	Extras map[string]any
}

// PublishOption 单次发布的选项函数。
type PublishOption func(*PublishConfig)

// WithKey 设置路由键/分区键。
func WithKey(key string) PublishOption {
	return func(c *PublishConfig) { c.Key = key }
}

// WithHeaders 追加消息头（与 Message.Headers 合并）。
func WithHeaders(h map[string]string) PublishOption {
	return func(c *PublishConfig) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		for k, v := range h {
			c.Headers[k] = v
		}
	}
}

// WithDelay 延时投递。不支持的后端返回 ErrNotSupported。
func WithDelay(d time.Duration) PublishOption {
	return func(c *PublishConfig) { c.Delay = d }
}

// WithTimeout 单次发送超时。
func WithTimeout(d time.Duration) PublishOption {
	return func(c *PublishConfig) { c.Timeout = d }
}

// ── SubscribeConfig & SubscribeOption ──

// SubscribeConfig 单次订阅的可选配置。
type SubscribeConfig struct {
	Group       string // 消费者组名
	Concurrency int    // 并发 goroutine 数，默认 1
	// Prefetch 预取数量（per consumer），默认 1。
	// 并发模式下每个 goroutine 独立持有 Prefetch 配额，总 inflight = Concurrency × Prefetch。
	// 如需控制总 inflight，调用方可自行计算：WithPrefetch(totalPrefetch / concurrency)。
	Prefetch int
	DLQTopic string // 死信目标 topic，空=不启用
	AutoAck  bool   // 自动确认，默认 true
	// ClaimIdle 用于 Redis Stream 消费者启动时 reclaim 其他 consumer 已闲置超过此时长的 pending 消息。
	// 设为 0（默认）表示跳过 reclaim，仅消费新消息。
	ClaimIdle time.Duration
}

// SubscribeOption 单次订阅的选项函数。
type SubscribeOption func(*SubscribeConfig)

// WithGroup 设置消费者组名。
func WithGroup(group string) SubscribeOption {
	return func(c *SubscribeConfig) { c.Group = group }
}

// WithConcurrency 设置并发 handler 数。
func WithConcurrency(n int) SubscribeOption {
	return func(c *SubscribeConfig) { c.Concurrency = n }
}

// WithPrefetch 设置预取数量（流控窗口）。
func WithPrefetch(n int) SubscribeOption {
	return func(c *SubscribeConfig) { c.Prefetch = n }
}

// WithDLQ 设置死信目标 topic。
func WithDLQ(topic string) SubscribeOption {
	return func(c *SubscribeConfig) { c.DLQTopic = topic }
}

// WithAutoAck 设置是否自动确认。
func WithAutoAck(v bool) SubscribeOption {
	return func(c *SubscribeConfig) { c.AutoAck = v }
}

// WithClaimIdle 设置 Redis Stream 消费者启动时 reclaim pending 消息的最小空闲时间。
// 设为 0 表示跳过 reclaim。仅 Redis Stream 适配器有效，其他适配器忽略。
func WithClaimIdle(d time.Duration) SubscribeOption {
	return func(c *SubscribeConfig) { c.ClaimIdle = d }
}

// Logger 日志接口。各适配器组件使用它记录内部事件（DLQ 写入失败、重连等）。
// 调用方注入自己的 logger 实现（如 slog、zap），nil 表示不记录。
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

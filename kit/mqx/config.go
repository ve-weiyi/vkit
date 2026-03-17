package mqx

import "time"

// ConsumerConfig 消费者配置（通用）
type ConsumerConfig struct {
	// 基础配置
	GroupID      string   // 消费者组ID（Kafka必需）
	Topics       []string // 订阅的主题/队列列表
	ConsumerName string   // 消费者名称（用于日志和监控）

	// 消费模式
	AutoCommit        bool          // 是否自动提交偏移量（Kafka）
	CommitInterval    time.Duration // 自动提交间隔（Kafka）
	SessionTimeout    time.Duration // 会话超时时间
	HeartbeatInterval time.Duration // 心跳间隔

	// 并发控制
	Concurrency   int // 并发消费者数量
	PrefetchCount int // 预取消息数量（RabbitMQ/Redis）

	// 重试配置
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试延迟
	EnableDLQ  bool          // 是否启用死信队列
	DLQTopic   string        // 死信队列主题

	// 消费起始位置（Kafka）
	StartOffset OffsetPosition // earliest, latest, specific

	// 其他配置
	Extra map[string]interface{} // 特定MQ的额外配置
}

// OffsetPosition 偏移量位置
type OffsetPosition string

const (
	OffsetEarliest OffsetPosition = "earliest" // 从最早开始
	OffsetLatest   OffsetPosition = "latest"   // 从最新开始
	OffsetSpecific OffsetPosition = "specific" // 指定偏移量
)

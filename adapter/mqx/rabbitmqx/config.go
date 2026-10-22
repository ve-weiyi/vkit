package rabbitmqx

import (
	"crypto/tls"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Exchange 类型常量。
const (
	ExchangeDirect  = "direct"
	ExchangeTopic   = "topic"
	ExchangeFanout  = "fanout"
	ExchangeHeaders = "headers"
)

// Config RabbitMQ 连接与拓扑配置。
type Config struct {
	// ── 连接 ──
	URL       string
	TLS       *tls.Config
	VHost     string
	Heartbeat time.Duration

	// ── Exchange 拓扑 ──
	ExchangeName string
	ExchangeType string // ExchangeDirect / ExchangeTopic / ExchangeFanout / ExchangeHeaders，默认 ExchangeTopic
	Durable      bool
	AutoDelete   bool

	// ── 重连 ──
	Reconnect ReconnectConfig

	// LazyConnect 为 true 时 New 不等待首次连接，立即返回并在后台按 Reconnect
	// 配置持续重试。发布器/订阅器在连接就绪前可正常创建：发布返回
	// ErrNotConnected，订阅会等待首次连接后自动开始消费。
	LazyConnect bool

	// ── 日志 ──
	// Logger 用于记录内部事件（DLQ 写入失败等）。nil 表示不记录。
	Logger mqx.Logger
}

// ReconnectConfig 自动重连配置。
type ReconnectConfig struct {
	MaxRetries   int           // 最大重试次数，0 = 无限
	InitialDelay time.Duration // 初始重连间隔，默认 1s
	MaxDelay     time.Duration // 最大重连间隔，默认 30s
}

func (c *Config) normalize() {
	if c.ExchangeType == "" {
		c.ExchangeType = ExchangeTopic
	}
	if c.Heartbeat == 0 {
		c.Heartbeat = 30 * time.Second
	}
	if c.Reconnect.InitialDelay == 0 {
		c.Reconnect.InitialDelay = 1 * time.Second
	}
	if c.Reconnect.MaxDelay == 0 {
		c.Reconnect.MaxDelay = 30 * time.Second
	}
}

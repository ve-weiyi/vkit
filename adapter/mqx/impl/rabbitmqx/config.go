package rabbitmqx

import (
	"crypto/tls"
	"log/slog"
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

// Config 是 RabbitMQ 连接与行为配置。
type Config struct {
	// URL 是 AMQP 连接串；VHost 非空时覆盖 URL 中的 path。
	URL       string
	VHost     string
	TLS       *tls.Config
	Heartbeat time.Duration

	// ExchangeType 是发布/订阅时声明的交换机类型，默认 topic。
	ExchangeType string
	// Transient 为 true 时 exchange/queue 非持久化且自动删除；默认持久化。
	Transient bool

	// PublisherPoolSize 是发布确认 channel 数量，默认 4。
	PublisherPoolSize int

	Reconnect ReconnectConfig
	// LazyConnect 为 true 时 New 不等待首次连接，后台按 Reconnect 配置持续重试。
	LazyConnect bool

	Logger  *slog.Logger
	Metrics mqx.Metrics
}

// ReconnectConfig 是自动重连配置。
type ReconnectConfig struct {
	// MaxRetries 是最大重试次数，0 表示无限。
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

func (c *Config) normalize() {
	if c.URL == "" {
		c.URL = "amqp://guest:guest@localhost:5672/"
	}
	if c.Heartbeat == 0 {
		c.Heartbeat = 30 * time.Second
	}
	if c.ExchangeType == "" {
		c.ExchangeType = ExchangeTopic
	}
	if c.PublisherPoolSize <= 0 {
		c.PublisherPoolSize = 4
	}
	if c.Reconnect.InitialDelay <= 0 {
		c.Reconnect.InitialDelay = time.Second
	}
	if c.Reconnect.MaxDelay <= 0 {
		c.Reconnect.MaxDelay = 30 * time.Second
	}
	c.Logger = mqx.LoggerOrDiscard(c.Logger)
}

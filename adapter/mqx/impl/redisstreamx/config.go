package redisstreamx

import (
	"log/slog"
	"runtime"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Config 是 Redis Stream 连接与消费配置。
type Config struct {
	Addrs        []string
	Username     string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// ReadCount 是单次 XREADGROUP 拉取条数，默认 10。
	ReadCount int64
	// BlockTimeout 是 XREADGROUP 阻塞时长，默认 1s。
	BlockTimeout time.Duration
	// ClaimIdle 大于 0 时，订阅会周期性 XAUTOCLAIM 闲置超过该时长的 pending 消息。
	ClaimIdle time.Duration

	// LazyConnect 为 true 时构造期不 Ping，连接问题留到首次使用暴露。
	LazyConnect bool

	Logger  *slog.Logger
	Metrics mqx.Metrics
}

func (c *Config) normalize() {
	if len(c.Addrs) == 0 {
		c.Addrs = []string{"localhost:6379"}
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10 * runtime.GOMAXPROCS(0)
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.ReadCount <= 0 {
		c.ReadCount = 10
	}
	if c.BlockTimeout == 0 {
		c.BlockTimeout = time.Second
	}
	c.Logger = mqx.LoggerOrDiscard(c.Logger)
}

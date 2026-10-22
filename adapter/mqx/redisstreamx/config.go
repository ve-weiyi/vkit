package redisstreamx

import (
	"runtime"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Config Redis Stream 连接配置。支持单机/集群/哨兵自动检测。
type Config struct {
	// Addrs Redis 地址列表。单机模式传入单个地址，集群/哨兵传入多个。
	// 默认 {"localhost:6379"}。
	Addrs []string

	Password string
	DB       int // 单机模式使用的数据库编号

	// PoolSize 连接池大小。默认 10 * runtime.GOMAXPROCS(0)。
	PoolSize int
	// MinIdleConns 最小空闲连接数。
	MinIdleConns int
	// MaxRetries 命令级最大重试次数。默认 3。
	MaxRetries int

	// DialTimeout 建连超时。默认 5s。
	DialTimeout time.Duration
	// ReadTimeout 读超时。默认 3s。
	ReadTimeout time.Duration
	// WriteTimeout 写超时。默认 3s。
	WriteTimeout time.Duration

	// ── 日志 ──
	// Logger 用于记录内部事件（DLQ 写入失败等）。nil 表示不记录。
	Logger mqx.Logger
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
}

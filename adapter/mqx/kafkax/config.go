package kafkax

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Config Kafka 连接与行为配置。
type Config struct {
	// ── 连接 ──
	// Seeds Kafka broker 地址列表，如 {"localhost:9092"}。
	Seeds []string
	// TLS TLS 配置，nil = 明文连接。
	TLS *tls.Config
	// SASL 认证机制，nil = 无需认证。
	SASL SASL
	// DialTimeout 建连超时。默认 30s。
	DialTimeout time.Duration

	// ── 请求级重试 ──
	// MaxRetries 请求级最大重试次数。默认 3。
	MaxRetries int

	// ── 生产者默认配置 ──
	// RequiredAcks 消息确认级别。默认 AllISRAcks（最安全）。
	RequiredAcks Acks
	// Compression 压缩算法。默认不压缩。
	Compression Compression
	// BatchMaxBytes 单批最大字节数。默认 1MB。
	BatchMaxBytes int32
	// Linger 批次等待时间。默认 0（立即发送）。
	Linger time.Duration

	// ── 日志 ──
	// Logger 用于记录内部事件（DLQ 写入失败等）。nil 表示不记录。
	Logger mqx.Logger
}

func (c *Config) normalize() {
	if len(c.Seeds) == 0 {
		c.Seeds = []string{"localhost:9092"}
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 30 * time.Second
	}
	if c.BatchMaxBytes == 0 {
		c.BatchMaxBytes = 1_000_000
	}
}

// ── Acks ──

// Acks 消息确认级别。通过 AllISRAcks / LeaderAck / NoAck 构造。
type Acks struct {
	val kgo.Acks
}

// AllISRAcks 等待所有 ISR 副本确认，最安全。
func AllISRAcks() Acks { return Acks{val: kgo.AllISRAcks()} }

// LeaderAck 仅等待 leader 确认。
func LeaderAck() Acks { return Acks{val: kgo.LeaderAck()} }

// NoAck 不等待确认，最高吞吐但可能丢消息。
func NoAck() Acks { return Acks{val: kgo.NoAck()} }

// ── Compression ──

// Compression 压缩算法。通过 GzipCompression 等函数构造。
type Compression struct {
	codec kgo.CompressionCodec
}

// NoCompression 不压缩。
func NoCompression() Compression { return Compression{kgo.NoCompression()} }

// GzipCompression GZIP 压缩。
func GzipCompression() Compression { return Compression{kgo.GzipCompression()} }

// SnappyCompression Snappy 压缩。
func SnappyCompression() Compression { return Compression{kgo.SnappyCompression()} }

// Lz4Compression LZ4 压缩。
func Lz4Compression() Compression { return Compression{kgo.Lz4Compression()} }

// ZstdCompression Zstandard 压缩。
func ZstdCompression() Compression { return Compression{kgo.ZstdCompression()} }

// ── SASL ──

// SASL 认证机制接口。通过 PlainAuth / ScramAuth 构造函数创建。
type SASL interface {
	mechanism() kgo.Opt
}

type plainSASL struct{ user, pass, zid string }

// PlainAuth 创建 PLAIN 认证。
func PlainAuth(user, password string) SASL {
	return plainSASL{user: user, pass: password}
}

func (p plainSASL) mechanism() kgo.Opt {
	return kgo.SASL(plain.Plain(func(_ context.Context) (plain.Auth, error) {
		return plain.Auth{Zid: p.zid, User: p.user, Pass: p.pass}, nil
	}))
}

// ScramHash SCRAM 哈希算法。
type ScramHash int

const (
	// ScramSHA256 SCRAM-SHA-256。
	ScramSHA256 ScramHash = iota + 1
	// ScramSHA512 SCRAM-SHA-512。
	ScramSHA512
)

type scramSASL struct {
	user, pass string
	hash       ScramHash
}

// ScramAuth 创建 SCRAM 认证。hash 取 ScramSHA256 或 ScramSHA512。
func ScramAuth(user, password string, hash ScramHash) SASL {
	return scramSASL{user: user, pass: password, hash: hash}
}

func (s scramSASL) mechanism() kgo.Opt {
	switch s.hash {
	case ScramSHA512:
		return kgo.SASL(scram.Sha512(func(_ context.Context) (scram.Auth, error) {
			return scram.Auth{User: s.user, Pass: s.pass}, nil
		}))
	default: // ScramSHA256 and unknown values
		return kgo.SASL(scram.Sha256(func(_ context.Context) (scram.Auth, error) {
			return scram.Auth{User: s.user, Pass: s.pass}, nil
		}))
	}
}

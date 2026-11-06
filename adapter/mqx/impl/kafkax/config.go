package kafkax

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// Config 是 Kafka 连接与行为配置。
type Config struct {
	Seeds       []string
	TLS         *tls.Config
	SASL        SASL
	DialTimeout time.Duration
	MaxRetries  int

	// RequiredAcks 未设置时默认 all（与幂等生产者兼容）。
	RequiredAcks Acks
	// Compression 未设置时使用 franz-go 默认压缩。
	Compression   Compression
	BatchMaxBytes int32
	Linger        time.Duration

	Logger  *slog.Logger
	Metrics mqx.Metrics
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
	if !c.RequiredAcks.set {
		c.RequiredAcks = AllISRAcks()
	}
	c.Logger = mqx.LoggerOrDiscard(c.Logger)
}

func (c *Config) commonOpts() []kgo.Opt {
	opts := []kgo.Opt{
		kgo.SeedBrokers(c.Seeds...),
		kgo.DialTimeout(c.DialTimeout),
		kgo.RequestRetries(c.MaxRetries),
	}
	if c.TLS != nil {
		opts = append(opts, kgo.DialTLSConfig(c.TLS))
	}
	if c.SASL != nil {
		opts = append(opts, c.SASL.mechanism())
	}
	return opts
}

func (c *Config) producerOpts() []kgo.Opt {
	opts := c.commonOpts()
	opts = append(opts,
		kgo.RequiredAcks(c.RequiredAcks.val),
		kgo.ProducerBatchMaxBytes(c.BatchMaxBytes),
		kgo.ProducerLinger(c.Linger),
	)
	if c.Compression.set {
		opts = append(opts, kgo.ProducerBatchCompression(c.Compression.codec))
	}
	return opts
}

func (c *Config) consumerOpts(group, topic string) []kgo.Opt {
	opts := c.commonOpts()
	opts = append(opts,
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	return opts
}

// Acks 消息确认级别。零值表示未设置，由 normalize 补为 AllISRAcks。
type Acks struct {
	val kgo.Acks
	set bool
}

// AllISRAcks 等待所有同步副本确认。
func AllISRAcks() Acks { return Acks{val: kgo.AllISRAcks(), set: true} }

// LeaderAck 只等待 leader 确认。
func LeaderAck() Acks { return Acks{val: kgo.LeaderAck(), set: true} }

// NoAck 不等待确认。
func NoAck() Acks { return Acks{val: kgo.NoAck(), set: true} }

// Compression 压缩算法。零值表示未设置，交由 franz-go 选择默认算法。
type Compression struct {
	codec kgo.CompressionCodec
	set   bool
}

// NoCompression 关闭压缩。
func NoCompression() Compression { return Compression{codec: kgo.NoCompression(), set: true} }

// GzipCompression 使用 gzip。
func GzipCompression() Compression { return Compression{codec: kgo.GzipCompression(), set: true} }

// SnappyCompression 使用 snappy。
func SnappyCompression() Compression { return Compression{codec: kgo.SnappyCompression(), set: true} }

// Lz4Compression 使用 lz4。
func Lz4Compression() Compression { return Compression{codec: kgo.Lz4Compression(), set: true} }

// ZstdCompression 使用 zstd。
func ZstdCompression() Compression { return Compression{codec: kgo.ZstdCompression(), set: true} }

// SASL 认证机制。
type SASL interface {
	mechanism() kgo.Opt
}

type plainSASL struct{ user, pass string }

// PlainAuth 使用 SASL/PLAIN。
func PlainAuth(user, password string) SASL {
	return plainSASL{user: user, pass: password}
}

func (p plainSASL) mechanism() kgo.Opt {
	return kgo.SASL(plain.Plain(func(context.Context) (plain.Auth, error) {
		return plain.Auth{User: p.user, Pass: p.pass}, nil
	}))
}

// ScramHash 是 SCRAM 哈希算法。
type ScramHash int

const (
	// ScramSHA256 使用 SCRAM-SHA-256。
	ScramSHA256 ScramHash = iota + 1
	// ScramSHA512 使用 SCRAM-SHA-512。
	ScramSHA512
)

type scramSASL struct {
	user, pass string
	hash       ScramHash
}

// ScramAuth 使用 SASL/SCRAM。
func ScramAuth(user, password string, hash ScramHash) SASL {
	return scramSASL{user: user, pass: password, hash: hash}
}

func (s scramSASL) mechanism() kgo.Opt {
	if s.hash == ScramSHA512 {
		return kgo.SASL(scram.Sha512(func(context.Context) (scram.Auth, error) {
			return scram.Auth{User: s.user, Pass: s.pass}, nil
		}))
	}
	return kgo.SASL(scram.Sha256(func(context.Context) (scram.Auth, error) {
		return scram.Auth{User: s.user, Pass: s.pass}, nil
	}))
}

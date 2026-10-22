package kafkax

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

var _ mqx.Publisher = (*Publisher)(nil)

// Publisher Kafka 消息发布器。基于 franz-go ProduceSync。
// 不同 Ack 级别的请求使用独立 Client（最多 3 个，惰性创建）。
type Publisher struct {
	cfg    *Config
	closed atomic.Bool

	defaultAcks Acks
	defaultOpts []mqx.PublishOption

	// cacheMu 保护 clientCache 的并发访问。
	cacheMu     sync.RWMutex
	clientCache map[Acks]*kgo.Client
}

func newPublisher(cfg *Config, opts []mqx.PublishOption) (*Publisher, error) {
	p := &Publisher{
		cfg:         cfg,
		defaultAcks: cfg.RequiredAcks,
		defaultOpts: opts,
		clientCache: make(map[Acks]*kgo.Client),
	}

	// 预创建默认 Ack 级别的 Client 以尽早发现连接问题
	if _, err := p.getOrCreateClient(p.defaultAcks); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Publisher) producerOpts(acks Acks) []kgo.Opt {
	opts := []kgo.Opt{
		kgo.SeedBrokers(p.cfg.Seeds...),
		kgo.RequiredAcks(acks.val),
		kgo.RecordRetries(p.cfg.MaxRetries),
		kgo.AllowAutoTopicCreation(),
		kgo.ProducerBatchCompression(p.cfg.Compression.codec),
		kgo.ProducerBatchMaxBytes(p.cfg.BatchMaxBytes),
		kgo.ProducerLinger(p.cfg.Linger),
	}
	if p.cfg.TLS != nil {
		opts = append(opts, kgo.DialTLSConfig(p.cfg.TLS))
	}
	if p.cfg.SASL != nil {
		if m := p.cfg.SASL.mechanism(); m != nil {
			opts = append(opts, m)
		}
	}
	if p.cfg.DialTimeout > 0 {
		opts = append(opts, kgo.DialTimeout(p.cfg.DialTimeout))
	}
	return opts
}

// getOrCreateClient 获取或惰性创建指定 Ack 级别的 Client。使用 cacheMu 独立于主锁。
func (p *Publisher) getOrCreateClient(acks Acks) (*kgo.Client, error) {
	// 读锁快速路径
	p.cacheMu.RLock()
	if c, ok := p.clientCache[acks]; ok {
		p.cacheMu.RUnlock()
		return c, nil
	}
	p.cacheMu.RUnlock()

	// 写锁创建路径 — Ping 在此处执行，不阻塞 Publish 级别的调用
	p.cacheMu.Lock()
	// double-check
	if c, ok := p.clientCache[acks]; ok {
		p.cacheMu.Unlock()
		return c, nil
	}

	client, err := kgo.NewClient(p.producerOpts(acks)...)
	if err != nil {
		p.cacheMu.Unlock()
		return nil, fmt.Errorf("kafka: create producer client: %w", err)
	}

	if err := client.Ping(context.Background()); err != nil {
		p.cacheMu.Unlock()
		client.Close()
		return nil, fmt.Errorf("%w: %v", mqx.ErrNotConnected, err)
	}

	// double-check closed 后再写入 map，避免并发 Close 导致的 nil map write
	if p.closed.Load() {
		p.cacheMu.Unlock()
		client.Close()
		return nil, mqx.ErrClosed
	}

	p.clientCache[acks] = client
	p.cacheMu.Unlock()

	return client, nil
}

// resolveAcks 解析当前 Publish 应使用的 Ack 级别。
func (p *Publisher) resolveAcks(cfg *mqx.PublishConfig) Acks {
	if cfg.Extras == nil {
		return p.defaultAcks
	}
	if v, ok := cfg.Extras["kafkax:acks"]; ok {
		if a, ok := v.(Acks); ok {
			return a
		}
	}
	return p.defaultAcks
}

// Publish 实现 mqx.Publisher。使用 ProduceSync 同步发送单条消息。
func (p *Publisher) Publish(ctx context.Context, topic string, msg *mqx.Message, opts ...mqx.PublishOption) error {
	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Delay > 0 {
		return fmt.Errorf("%w: delayed publish is not supported with Kafka", mqx.ErrNotSupported)
	}

	acks := p.resolveAcks(cfg)

	if p.closed.Load() {
		return mqx.ErrClosed
	}

	client, err := p.getOrCreateClient(acks)
	if err != nil {
		return err
	}

	record := toKafkaRecord(msg, cfg.Headers)
	record.Topic = topic
	if cfg.Key != "" {
		record.Key = []byte(cfg.Key)
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	results := client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrSendFailed, err)
	}

	return nil
}

// PublishBatch 实现 mqx.Publisher。使用 ProduceSync 批量发送。
func (p *Publisher) PublishBatch(ctx context.Context, topic string, msgs []*mqx.Message, opts ...mqx.PublishOption) error {
	if len(msgs) == 0 {
		return nil
	}

	cfg := &mqx.PublishConfig{}
	for _, o := range p.defaultOpts {
		o(cfg)
	}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Delay > 0 {
		return fmt.Errorf("%w: delayed publish is not supported with Kafka", mqx.ErrNotSupported)
	}

	acks := p.resolveAcks(cfg)

	if p.closed.Load() {
		return mqx.ErrClosed
	}

	client, err := p.getOrCreateClient(acks)
	if err != nil {
		return err
	}

	records := make([]*kgo.Record, len(msgs))
	for i, msg := range msgs {
		r := toKafkaRecord(msg, cfg.Headers)
		r.Topic = topic
		if cfg.Key != "" {
			r.Key = []byte(cfg.Key)
		}
		records[i] = r
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	results := client.ProduceSync(ctx, records...)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("%w: %v", mqx.ErrSendFailed, err)
	}

	return nil
}

// Close 实现 mqx.Publisher。关闭所有缓存的 Client。
func (p *Publisher) Close() error {
	if p.closed.Swap(true) {
		return nil
	}

	p.cacheMu.Lock()
	for _, client := range p.clientCache {
		client.Close()
	}
	// 不将 clientCache 设为 nil，保留空 map 以便并发 getOrCreateClient 安全写入
	p.clientCache = make(map[Acks]*kgo.Client)
	p.cacheMu.Unlock()
	return nil
}

// WithKafkaAcks 设置本次发布的 Kafka 确认级别（覆盖 Config 默认值）。
// 不同 Ack 级别使用独立连接（惰性缓存），首次使用某级别时需额外建连开销。
func WithKafkaAcks(acks Acks) mqx.PublishOption {
	return func(c *mqx.PublishConfig) {
		if c.Extras == nil {
			c.Extras = make(map[string]any)
		}
		c.Extras["kafkax:acks"] = acks
	}
}

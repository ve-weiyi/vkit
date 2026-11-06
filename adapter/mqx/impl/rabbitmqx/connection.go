package rabbitmqx

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// rabbitMQConn 是线程安全的 AMQP 连接封装，支持自动重连。
type rabbitMQConn struct {
	mu        sync.RWMutex
	conn      *amqp.Connection
	url       string
	vhost     string
	tlsConf   *tls.Config
	heartbeat time.Duration
	reconnect ReconnectConfig
	logger    *slog.Logger

	generation atomic.Uint64

	closeCh     chan struct{}
	doneCh      chan struct{}
	notifyChs   []chan struct{}
	permCloseCh chan struct{}
	permOnce    sync.Once

	closed bool
}

// dialConn 建立连接并启动连接维护循环。
// LazyConnect 为 true 时不在调用方 goroutine 里拨号。
func dialConn(cfg *Config) (*rabbitMQConn, error) {
	rc := &rabbitMQConn{
		url:         cfg.URL,
		vhost:       cfg.VHost,
		tlsConf:     cfg.TLS,
		heartbeat:   cfg.Heartbeat,
		reconnect:   cfg.Reconnect,
		logger:      cfg.Logger,
		closeCh:     make(chan struct{}),
		doneCh:      make(chan struct{}),
		permCloseCh: make(chan struct{}),
	}

	if !cfg.LazyConnect {
		conn, err := dial(cfg.URL, cfg.VHost, cfg.TLS, cfg.Heartbeat)
		if err != nil {
			return nil, fmt.Errorf("rabbitmqx: dial: %w", err)
		}
		rc.conn = conn
		rc.generation.Store(1)
	}

	go rc.connectionLoop()
	return rc, nil
}

func dial(rawURL, vhost string, tlsConf *tls.Config, heartbeat time.Duration) (*amqp.Connection, error) {
	if vhost != "" {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}
		u.Path = "/" + vhost
		rawURL = u.String()
	}
	cfg := amqp.Config{Heartbeat: heartbeat, Locale: "en_US"}
	if tlsConf != nil {
		cfg.TLSClientConfig = tlsConf
	}
	return amqp.DialConfig(rawURL, cfg)
}

// Channel 返回一个新的 amqp.Channel。
func (c *rabbitMQConn) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil || c.conn.IsClosed() {
		return nil, mqx.ErrUnavailable
	}
	return c.conn.Channel()
}

// Generation 在每次重连成功后递增，供发布器判断是否需要重建 channel。
func (c *rabbitMQConn) Generation() uint64 { return c.generation.Load() }

// NotifyReconnect 注册重连通知 channel；不再需要时调用 UnregisterReconnect。
func (c *rabbitMQConn) NotifyReconnect() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan struct{}, 1)
	c.notifyChs = append(c.notifyChs, ch)
	return ch
}

// UnregisterReconnect 注销并关闭通知 channel。
func (c *rabbitMQConn) UnregisterReconnect(ch chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, registered := range c.notifyChs {
		if registered != ch {
			continue
		}
		c.notifyChs = append(c.notifyChs[:i], c.notifyChs[i+1:]...)
		select {
		case <-ch:
		default:
			close(ch)
		}
		return
	}
}

// Ping 检测连接是否存活。
func (c *rabbitMQConn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil || c.conn.IsClosed() {
		return mqx.ErrUnavailable
	}
	return nil
}

// NotifyPermanentClose 在重连重试耗尽时关闭。
func (c *rabbitMQConn) NotifyPermanentClose() <-chan struct{} {
	return c.permCloseCh
}

// Close 关闭连接并停止重连。
func (c *rabbitMQConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.closeCh)
	<-c.doneCh

	c.mu.Lock()
	for _, ch := range c.notifyChs {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	c.notifyChs = nil
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (c *rabbitMQConn) notifyReconnect() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, ch := range c.notifyChs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (c *rabbitMQConn) connectionLoop() {
	defer close(c.doneCh)

	if c.current() == nil {
		if !c.connectWithRetry() {
			return
		}
		c.notifyReconnect()
	}

	for {
		conn := c.current()
		if conn == nil {
			return
		}

		select {
		case <-c.closeCh:
			return
		case <-conn.NotifyClose(make(chan *amqp.Error, 1)):
		}

		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()

		select {
		case <-c.closeCh:
			return
		default:
		}

		if !c.connectWithRetry() {
			return
		}
		c.notifyReconnect()
	}
}

func (c *rabbitMQConn) current() *amqp.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *rabbitMQConn) connectWithRetry() bool {
	for attempt := 0; ; attempt++ {
		select {
		case <-c.closeCh:
			return false
		default:
		}

		conn, err := dial(c.url, c.vhost, c.tlsConf, c.heartbeat)
		if err == nil {
			c.mu.Lock()
			c.conn = conn
			c.mu.Unlock()
			c.generation.Add(1)
			return true
		}
		c.logger.Error("mqx: rabbitmq dial failed", "attempt", attempt, "error", err.Error())

		if c.reconnect.MaxRetries > 0 && attempt+1 >= c.reconnect.MaxRetries {
			c.permOnce.Do(func() { close(c.permCloseCh) })
			return false
		}

		timer := time.NewTimer(backoffDelay(c.reconnect.InitialDelay, c.reconnect.MaxDelay, attempt))
		select {
		case <-c.closeCh:
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

func backoffDelay(initial, max time.Duration, attempt int) time.Duration {
	d := float64(initial) * math.Pow(2, float64(attempt))
	if d > float64(max) {
		d = float64(max)
	}
	return time.Duration(d)
}

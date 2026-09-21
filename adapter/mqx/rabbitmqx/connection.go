package rabbitmqx

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// rabbitMQConn 线程安全的 AMQP 连接封装。
// 支持自动重连：断连后指数退避重试，成功后通知所有依赖方。
type rabbitMQConn struct {
	mu        sync.RWMutex
	conn      *amqp.Connection
	url       string
	vhost     string
	tlsConf   *tls.Config
	heartbeat time.Duration
	reconnect ReconnectConfig

	closeCh     chan struct{}   // 外部请求关闭
	doneCh      chan struct{}   // 重连 goroutine 已退出
	notifyChs   []chan struct{} // 重连成功通知（publisher/subscriber 注册）
	permCloseCh chan struct{}   // 重连耗尽时关闭
	permOnce    sync.Once       // 确保 permCloseCh 只关闭一次

	closed bool
}

// dialConn 建立到 RabbitMQ 的连接并启动自动重连。
func dialConn(cfg *Config) (*rabbitMQConn, error) {
	cfg.normalize()
	conn, err := dial(cfg.URL, cfg.VHost, cfg.TLS, cfg.Heartbeat)
	if err != nil {
		return nil, fmt.Errorf("rabbitmqx: dial %s: %w", cfg.URL, err)
	}

	rc := &rabbitMQConn{
		conn:        conn,
		url:         cfg.URL,
		vhost:       cfg.VHost,
		tlsConf:     cfg.TLS,
		heartbeat:   cfg.Heartbeat,
		reconnect:   cfg.Reconnect,
		closeCh:     make(chan struct{}),
		doneCh:      make(chan struct{}),
		permCloseCh: make(chan struct{}),
	}
	go rc.reconnectLoop()
	return rc, nil
}

// dial 建立单次连接。如果 vhost 非空，会覆盖 URL 中的 path（vhost）。
func dial(rawURL string, vhost string, tlsConf *tls.Config, heartbeat time.Duration) (*amqp.Connection, error) {
	if vhost != "" {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("rabbitmqx: parse url: %w", err)
		}
		u.Path = "/" + vhost
		rawURL = u.String()
	}
	cfg := amqp.Config{
		Heartbeat: heartbeat,
		Locale:    "en_US",
	}
	if tlsConf != nil {
		cfg.TLSClientConfig = tlsConf
	}
	return amqp.DialConfig(rawURL, cfg)
}

// Channel 返回一个新的 amqp.Channel。
func (c *rabbitMQConn) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return nil, mqx.ErrNotConnected
	}
	return c.conn.Channel()
}

// NotifyReconnect 注册重连通知 channel。每次重连成功会发送一个信号。
// 返回的 channel 应在不再需要时通过 UnregisterReconnect 注销，避免泄漏。
func (c *rabbitMQConn) NotifyReconnect() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan struct{}, 1)
	c.notifyChs = append(c.notifyChs, ch)
	return ch
}

// UnregisterReconnect 从通知列表中移除 channel 并关闭它。
// 安全关闭：若 channel 已被 conn.Close() 关闭则跳过，避免 double-close panic。
func (c *rabbitMQConn) UnregisterReconnect(ch chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, registered := range c.notifyChs {
		if registered == ch {
			c.notifyChs = append(c.notifyChs[:i], c.notifyChs[i+1:]...)
			// 检查 channel 是否已被关闭（如被 conn.Close() 提前关闭），避免 panic
			select {
			case <-ch:
				// channel 已关闭
			default:
				close(ch)
			}
			break
		}
	}
}

// Ping 检测连接是否存活。若 ctx 已取消则直接返回 ctx.Err()。
func (c *rabbitMQConn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil || c.conn.IsClosed() {
		return mqx.ErrNotConnected
	}
	return nil
}

// NotifyPermanentClose 返回一个 channel，当重连重试耗尽导致永久断开时关闭。
func (c *rabbitMQConn) NotifyPermanentClose() <-chan struct{} {
	return c.permCloseCh
}

// Close 关闭连接，停止重连。
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

	// 清理所有注册的重连通知 channel，避免泄漏
	c.mu.Lock()
	for _, ch := range c.notifyChs {
		close(ch)
	}
	c.notifyChs = nil
	c.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// notifyReconnect 广播重连通知。
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

// reconnectLoop 自动重连循环。指数退避直到 MaxDelay。
//
// NOTE: 每次迭代创建一个新的 NotifyClose listener。amqp091-go 不支持 listener 移除，
// 旧的 listener 会一直保留直到 GC。对于频繁重连的场景，建议设置较大的 MaxRetries。
func (c *rabbitMQConn) reconnectLoop() {
	defer close(c.doneCh)

	for {
		select {
		case <-c.closeCh:
			return
		default:
		}

		// 等待当前连接断开
		select {
		case <-c.closeCh:
			return
		case <-c.conn.NotifyClose(make(chan *amqp.Error, 1)):
		}

		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()

		select {
		case <-c.closeCh:
			return
		default:
		}

		// 指数退避重试
		for attempt := 0; c.reconnect.MaxRetries == 0 || attempt < c.reconnect.MaxRetries; attempt++ {
			delay := backoffDelay(c.reconnect.InitialDelay, c.reconnect.MaxDelay, attempt)

			timer := time.NewTimer(delay)
			select {
			case <-c.closeCh:
				timer.Stop()
				return
			case <-timer.C:
			}

			newConn, err := dial(c.url, c.vhost, c.tlsConf, c.heartbeat)
			if err != nil {
				continue
			}

			c.mu.Lock()
			c.conn = newConn
			c.mu.Unlock()

			c.notifyReconnect()
			break
		}

		c.mu.Lock()
		if c.conn == nil {
			c.mu.Unlock()
			// 重连重试耗尽，通知永久断开。
			c.permOnce.Do(func() { close(c.permCloseCh) })
			return
		}
		c.mu.Unlock()
	}
}

// backoffDelay 计算指数退避延迟。
func backoffDelay(initial, max time.Duration, attempt int) time.Duration {
	d := float64(initial) * math.Pow(2, float64(attempt))
	if d > float64(max) {
		d = float64(max)
	}
	return time.Duration(d)
}

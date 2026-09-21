package httpx

import (
	"io"
	"net"
	"net/http"
	"time"
)

// DefaultTransport 返回默认的 Transport 配置。
func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// WithTransport 设置自定义 Transport。
func WithTransport(transport *http.Transport) Option {
	return func(c *config) {
		c.client.transport = transport
	}
}

// WithInsecureSkipVerify 跳过 TLS 证书验证。
func WithInsecureSkipVerify() Option {
	return func(c *config) {
		c.client.insecureSkipVerify = true
	}
}

// WithLoggingTransport 在 Transport 层启用详细日志（headers/body/streaming chunks）。
// 适合调试和问题排查。若只需一行摘要日志，使用 LoggerMiddleware。
// 输出到 w；若 w 为 nil，默认输出到 os.Stderr。
func WithLoggingTransport(w io.Writer) Option {
	return func(c *config) {
		c.client.loggingTransport = w
	}
}

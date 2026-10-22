package httpx

import (
	"context"
	"time"
)

// Logger 最小日志接口，兼容 *log.Logger。
type Logger interface {
	Printf(format string, v ...any)
}

// LoggerMiddleware 记录请求/响应一行摘要日志（方法、路径、状态码、耗时）。
// 适合生产环境请求追踪。如需详细 headers/body 日志，使用 WithLoggingTransport。
func LoggerMiddleware(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start)

			if err != nil {
				logger.Printf("%s %s -> error: %v (attempt %d, %s)",
					req.Method, req.URL.Path, err, req.Attempt, duration)
			} else {
				logger.Printf("%s %s -> %d (attempt %d, %s)",
					req.Method, req.URL.Path, resp.StatusCode, req.Attempt, duration)
			}

			return resp, err
		}
	}
}

package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

// RetryMiddleware 返回一个按 RetryConfig 自动重试的中间件。
// 若 MaxAttempts <= 1，返回一个直接透传的中间件（不重试）。
func RetryMiddleware(cfg RetryConfig) Middleware {
	if cfg.MaxAttempts <= 1 {
		return func(next Handler) Handler {
			return next
		}
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			return doWithRetry(ctx, next, req, cfg)
		}
	}
}

// doWithRetry 执行重试循环。
func doWithRetry(ctx context.Context, next Handler, req *Request, cfg RetryConfig) (*Response, error) {
	for {
		resp, err := next(ctx, req)
		if err == nil {
			return resp, nil
		}

		// 流式模式不重试（响应可能已部分消费）
		if req.Stream {
			return nil, err
		}

		// 达到最大尝试次数
		if req.Attempt >= cfg.MaxAttempts {
			return nil, err
		}

		// 不可重试的错误
		if !shouldRetry(err) {
			return nil, err
		}

		// 检查 context 是否取消
		if ctx.Err() != nil {
			return nil, err
		}

		// 重新创建请求体以便重放
		if req.GetBody != nil {
			rc, getBodyErr := req.GetBody()
			if getBodyErr != nil {
				return nil, fmt.Errorf("retry: getBody failed (%w) after request error: %w", getBodyErr, err)
			}
			newReq := req.Clone(ctx)
			newReq.Body = rc
			newReq.GetBody = req.GetBody
			req.Request = newReq
		}

		req.Attempt++

		if cfg.Delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(cfg.Delay):
			}
		}
	}
}

// shouldRetry 判断错误是否可重试：5xx 和网络错误可重试，4xx 不可重试。
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// context 取消或超时不重试
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// HTTP 错误：5xx 重试，4xx 不重试
	var httpErr *Error
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}

	// 网络错误可重试
	return isNetworkError(err)
}

// isNetworkError 检测是否为网络层错误，仅依赖类型断言。
func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}

	return isTimeout(err)
}

// isTimeout 检测错误是否由超时引起。
func isTimeout(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	var t timeout
	if errors.As(err, &t) {
		return t.Timeout()
	}
	return false
}

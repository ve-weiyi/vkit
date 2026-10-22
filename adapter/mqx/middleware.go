package mqx

import (
	"context"
	"fmt"
	"time"
)

// Middleware 包装 Handler，实现横切关注点。
type Middleware func(Handler) Handler

// Chain 将多个 Middleware 串联。执行顺序从左到右（第一个在最外层）。
func Chain(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logging 返回一个 Middleware：记录每次消息处理的耗时和结果。
// logger 为 nil 时，直接透传，无性能损耗。
func Logging(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, msg *Message) error {
			if logger == nil {
				return next(ctx, msg)
			}
			start := time.Now()
			err := next(ctx, msg)
			elapsed := time.Since(start)
			if err != nil {
				logger.Error("mqx: handler failed",
					"id", msg.ID,
					"key", msg.Key,
					"elapsed_ms", elapsed.Milliseconds(),
					"error", err.Error(),
				)
			} else {
				logger.Info("mqx: handler ok",
					"id", msg.ID,
					"key", msg.Key,
					"elapsed_ms", elapsed.Milliseconds(),
				)
			}
			return err
		}
	}
}

// Recovery 返回一个 Middleware：catch handler 中的 panic，转为 error 返回。
// panic 不会导致消费循环退出。
func Recovery(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, msg *Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if logger != nil {
						logger.Error("mqx: handler panicked",
							"panic", fmt.Sprintf("%v", r),
							"message_id", msg.ID,
						)
					}
					err = fmt.Errorf("mqx: handler panicked: %v", r)
				}
			}()
			return next(ctx, msg)
		}
	}
}

// MetricsRegistry 指标收集接口。
// 调用方实现此接口以接入 Prometheus、statsd 等监控系统。
type MetricsRegistry interface {
	// HandlerCompleted 记录一次 handler 执行完成。
	// dur 为执行耗时；err 为 handler 返回的错误（nil 表示成功）。
	HandlerCompleted(dur time.Duration, err error)
}

// Metrics 返回一个 Middleware：上报每次消息处理的耗时和结果。
// registry 为 nil 时直接透传。
func Metrics(registry MetricsRegistry) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, msg *Message) error {
			if registry == nil {
				return next(ctx, msg)
			}
			start := time.Now()
			err := next(ctx, msg)
			registry.HandlerCompleted(time.Since(start), err)
			return err
		}
	}
}

// Retry 返回一个 Middleware：handler 返回 error 时线性退避重试最多 maxRetries 次。
// maxRetries=0 表示不重试（仅尝试 1 次后直接返回结果）。
// maxRetries=N 表示最多重试 N 次，即总共最多 N+1 次 handler 调用（包含首次尝试）。
//
// 若 ctx 在重试等待期间被取消，立即返回 ctx.Err()，不再继续重试。
func Retry(maxRetries int) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, msg *Message) error {
			var err error
			for retries := 0; ; retries++ {
				err = next(ctx, msg)
				if err == nil {
					return nil
				}
				if ctx.Err() != nil {
					return fmt.Errorf("%w: %v", err, ctx.Err())
				}
				if retries >= maxRetries {
					return err
				}
				// 线性退避：100ms, 200ms, 300ms, ...
				timer := time.NewTimer(time.Duration(retries+1) * 100 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("%w: %v", err, ctx.Err())
				case <-timer.C:
				}
			}
		}
	}
}

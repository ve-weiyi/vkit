package mqx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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

// Logging 记录每次 handler 调用的耗时与结果。
func Logging(logger *slog.Logger) Middleware {
	logger = LoggerOrDiscard(logger)
	return func(next Handler) Handler {
		return func(ctx context.Context, d *Delivery) error {
			start := time.Now()
			err := next(ctx, d)
			attrs := []any{
				"destination", d.Destination,
				"routing_key", d.RoutingKey,
				"message_id", d.ID,
				"attempt", d.Attempt,
				"duration", time.Since(start).String(),
			}
			if err != nil {
				logger.ErrorContext(ctx, "mqx: handler failed", append(attrs, "error", err.Error())...)
			} else {
				logger.InfoContext(ctx, "mqx: handler ok", attrs...)
			}
			return err
		}
	}
}

// MetricsMiddleware 上报每次 handler 执行（含进程内重试的每一次尝试）。
func MetricsMiddleware(metrics Metrics, group string) Middleware {
	if metrics == nil {
		return func(next Handler) Handler { return next }
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, d *Delivery) (err error) {
			start := time.Now()
			defer func() {
				if r := recover(); r != nil {
					metrics.ConsumeCompleted(d.Destination, d.RoutingKey, group, d.Attempt, time.Since(start), fmt.Errorf("mqx: panic: %v", r))
					panic(r)
				}
				metrics.ConsumeCompleted(d.Destination, d.RoutingKey, group, d.Attempt, time.Since(start), err)
			}()
			return next(ctx, d)
		}
	}
}

// LoggerOrDiscard 在 logger 为 nil 时返回一个丢弃输出的 logger，避免每个调用点判空。
func LoggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

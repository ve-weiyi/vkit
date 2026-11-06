package event

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type messageHandler func(ctx context.Context, body []byte) error

// Router 在单个订阅上按 routing key 分发到类型化处理器。
type Router struct {
	sub     mqx.Subscriber
	logger  *slog.Logger
	metrics mqx.Metrics

	mu       sync.RWMutex
	handlers map[string]messageHandler

	wrapHandler func(mqx.Handler) mqx.Handler
}

// RouterOption 配置 Router 的可选行为。
type RouterOption func(*Router)

// WithLogger 设置 Router 日志器。
func WithLogger(logger *slog.Logger) RouterOption {
	return func(r *Router) { r.logger = logger }
}

func withHandlerWrapper(wrap func(mqx.Handler) mqx.Handler) RouterOption {
	return func(r *Router) { r.wrapHandler = wrap }
}

// NewRouter 创建路由器。
func NewRouter(sub mqx.Subscriber, opts ...RouterOption) *Router {
	router := &Router{
		sub:      sub,
		handlers: make(map[string]messageHandler),
	}
	for _, opt := range opts {
		opt(router)
	}
	return router
}

// HandleEvent 按事件类型声明的 routing key 注册处理器。
//
// 消息体反序列化失败视为毒消息：返回不可重试错误，交由适配器投递死信或记录丢弃。
func HandleEvent[T any](r *Router, spec EventSpec[T], handler func(context.Context, *T) error) error {
	if r == nil {
		return mqx.ErrUnavailable
	}
	if spec.Destination() == "" {
		return fmt.Errorf("%w: destination is required", mqx.ErrBadMessage)
	}
	key := spec.RoutingKey()
	if key == "" {
		return fmt.Errorf("%w: routing key is required", mqx.ErrBadMessage)
	}
	if handler == nil {
		return fmt.Errorf("%w: handler is required", mqx.ErrBadMessage)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[key]; exists {
		return fmt.Errorf("event: handler already registered for %s", key)
	}

	r.handlers[key] = func(ctx context.Context, body []byte) error {
		var event T
		if err := spec.Codec().Unmarshal(body, &event); err != nil {
			if r.logger != nil {
				r.logger.ErrorContext(ctx, "event: unmarshal message",
					"routing_key", key,
					"error", err.Error(),
				)
			}
			return mqx.Permanent(fmt.Errorf("event: decode %s: %w", key, err))
		}
		return handler(ctx, &event)
	}
	return nil
}

// Subscribe 用 Router 作为 handler 建立订阅。
func (r *Router) Subscribe(ctx context.Context, spec mqx.SubscriptionSpec) (mqx.Subscription, error) {
	if r == nil || r.sub == nil {
		return nil, mqx.ErrUnavailable
	}

	handler := func(ctx context.Context, d *mqx.Delivery) error {
		r.mu.RLock()
		next, ok := r.handlers[d.RoutingKey]
		r.mu.RUnlock()
		if !ok {
			// 未注册的事件类型不可重试：按订阅声明的失败去向处理（死信或显式丢弃），
			// 不能 Ack 了事，否则新事件类型会静默消失。
			return mqx.Permanent(fmt.Errorf("event: unhandled routing key %q", d.RoutingKey))
		}
		return next(withDelivery(ctx, d), d.Body)
	}

	middlewares := make([]mqx.Middleware, 0, 1)
	if r.logger != nil {
		middlewares = append(middlewares, mqx.Logging(r.logger))
	}
	if len(middlewares) > 0 {
		handler = mqx.Chain(middlewares...)(handler)
	}
	if r.wrapHandler != nil {
		handler = r.wrapHandler(handler)
	}
	return r.sub.Subscribe(ctx, spec, handler)
}

package event

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

const (
	// 订阅异常终止后的重建退避区间。
	restartInitialDelay = time.Second
	restartMaxDelay     = 30 * time.Second
	// 订阅存活超过该时长后视为稳定，重建退避回到起点。
	restartResetAfter = time.Minute
)

// RuntimeOption 配置事件运行时。
type RuntimeOption func(*Runtime)

// WithTracerProvider 为事件层注入 TracerProvider。
func WithTracerProvider(provider trace.TracerProvider) RuntimeOption {
	return func(r *Runtime) {
		if provider != nil {
			r.tracer = provider.Tracer("mqx")
		}
	}
}

// WithPropagator 指定 trace context 传播器，默认 W3C Trace Context + Baggage。
func WithPropagator(propagator propagation.TextMapPropagator) RuntimeOption {
	return func(r *Runtime) {
		if propagator != nil {
			r.propagator = propagator
		}
	}
}

// WithMessagingSystem 设置 messaging.system 属性。
func WithMessagingSystem(system string) RuntimeOption {
	return func(r *Runtime) {
		if system != "" {
			r.system = system
		}
	}
}

// Runtime 管理 Broker 与订阅生命周期。
type Runtime struct {
	broker mqx.Broker
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	closed bool
	subs   map[mqx.Subscription]struct{}

	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	system     string
}

// NewRuntime 创建事件运行时。
func NewRuntime(broker mqx.Broker, logger *slog.Logger, opts ...RuntimeOption) *Runtime {
	ctx, cancel := context.WithCancel(context.Background())
	tracing := defaultTracingConfig()
	runtime := &Runtime{
		broker:     broker,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		subs:       make(map[mqx.Subscription]struct{}),
		tracer:     tracing.tracer,
		propagator: tracing.propagator,
		system:     tracing.system,
	}
	for _, opt := range opts {
		opt(runtime)
	}
	return runtime
}

// NewPublisher 创建带 trace 传播的发布器。
//
// 每次调用返回独立包装；底层是否共享连接由适配器决定。
func (r *Runtime) NewPublisher() (mqx.Publisher, error) {
	pub, err := r.broker.Publisher()
	if err != nil {
		return nil, err
	}
	return wrapPublisher(pub, r.tracingConfig()), nil
}

// NewRouter 派生一个路由器。
func (r *Runtime) NewRouter(opts ...RouterOption) (*Router, error) {
	sub, err := r.broker.Subscriber()
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		WithLogger(r.logger),
		withHandlerWrapper(func(handler mqx.Handler) mqx.Handler {
			return wrapHandler(handler, r.tracingConfig())
		}),
	)
	return NewRouter(sub, opts...), nil
}

// Start 启动订阅并托管其生命周期：订阅异常终止后按指数退避重建，
// 直到 Runtime 关闭或 broker 拒绝重建。
func (r *Runtime) Start(router *Router, spec mqx.SubscriptionSpec) error {
	if router == nil {
		return mqx.ErrUnavailable
	}
	sub, err := router.Subscribe(r.ctx, spec)
	if err != nil {
		return err
	}
	if !r.track(sub) {
		_ = sub.Close(context.Background())
		return mqx.ErrClosed
	}

	go r.supervise(router, spec, sub)
	return nil
}

// track 记录订阅；Runtime 已关闭时返回 false。
func (r *Runtime) track(sub mqx.Subscription) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.subs[sub] = struct{}{}
	return true
}

// untrack 移除订阅并返回 Runtime 是否已关闭。
func (r *Runtime) untrack(sub mqx.Subscription) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, sub)
	return r.closed
}

// supervise 等待订阅终止，记录致命错误并按退避重建。
//
// 只重建、不无限热循环：重建间隔从 1s 指数增长到 30s；订阅稳定运行超过
// restartResetAfter 后退避归零。broker 已关闭时重建没有意义，直接退出。
func (r *Runtime) supervise(router *Router, spec mqx.SubscriptionSpec, sub mqx.Subscription) {
	logger := mqx.LoggerOrDiscard(r.logger)
	delay := restartInitialDelay

	for {
		startedAt := time.Now()
		errs := sub.Err()
		if errs == nil {
			// 后端未暴露错误通道：无法判断终止原因，重建只会空转。
			r.untrack(sub)
			return
		}
		cause := drainErrors(errs)
		if r.untrack(sub) {
			return
		}
		if time.Since(startedAt) >= restartResetAfter {
			delay = restartInitialDelay
		}
		if cause == nil {
			cause = mqx.ErrClosed
		}
		logger.Error("event: subscription stopped, restarting",
			"destination", spec.Destination,
			"group", spec.Group,
			"error", cause.Error(),
			"restart_in", delay.String(),
		)

		select {
		case <-r.ctx.Done():
			return
		case <-time.After(delay):
		}

		next, err := router.Subscribe(r.ctx, spec)
		if err != nil {
			logger.Error("event: subscription restart failed",
				"destination", spec.Destination,
				"group", spec.Group,
				"error", err.Error(),
			)
			if errors.Is(err, mqx.ErrClosed) {
				return
			}
			delay = restartBackoff(delay)
			continue
		}
		if !r.track(next) {
			_ = next.Close(context.Background())
			return
		}
		sub = next
		delay = restartBackoff(delay)
	}
}

// drainErrors 读空错误通道并返回最后一个非 nil 错误。
func drainErrors(errs <-chan error) error {
	var last error
	for err := range errs {
		if err != nil {
			last = err
		}
	}
	return last
}

func restartBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > restartMaxDelay {
		next = restartMaxDelay
	}
	return next
}

// Close 先停止全部订阅，再关闭 Broker。
func (r *Runtime) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	subs := make([]mqx.Subscription, 0, len(r.subs))
	for sub := range r.subs {
		subs = append(subs, sub)
	}
	r.subs = make(map[mqx.Subscription]struct{})
	r.mu.Unlock()

	r.cancel()
	for _, sub := range subs {
		if err := sub.Close(ctx); err != nil {
			mqx.LoggerOrDiscard(r.logger).Error("event: close subscription", "error", err.Error())
		}
	}
	return r.broker.Close(ctx)
}

func (r *Runtime) tracingConfig() tracingConfig {
	return tracingConfig{tracer: r.tracer, propagator: r.propagator, system: r.system}
}

// Logger 返回运行时日志器。
func (r *Runtime) Logger() *slog.Logger { return r.logger }

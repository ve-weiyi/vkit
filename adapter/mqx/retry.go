package mqx

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime/debug"
	"time"
)

// Process 调用 handler 并按 RetryPolicy 重试。
//
// 返回最终错误；适配器据此决定 Ack、Nack 或投递死信。
// handler 手动 Ack/Nack 后立即返回，不再重试。
// handler panic 会被捕获并转为不可重试错误，避免消费协程崩溃拖垮进程。
func Process(ctx context.Context, d *Delivery, handler Handler, policy RetryPolicy) error {
	attempts := policy.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}

	start := time.Now()
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		d.Attempt = attempt
		err = callHandler(ctx, d, handler)
		if err == nil || d.Settled() {
			return err
		}
		if IsPermanent(err) {
			return err
		}
		if policy.MaxElapsed > 0 && time.Since(start) >= policy.MaxElapsed {
			return err
		}
		if attempt == attempts {
			return err
		}

		delay := policy.delay(attempt)
		if custom, ok := RetryDelay(err); ok {
			delay = custom
			if policy.MaxDelay > 0 && delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}
		if delay <= 0 {
			continue
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

// callHandler 调用 handler 并把 panic 转为不可重试错误：重试只会重复触发同一个缺陷。
// 错误信息附带堆栈，最终丢弃日志里可直接定位 panic 位置。
func callHandler(ctx context.Context, d *Delivery, handler Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = Permanent(fmt.Errorf("mqx: handler panic: %v\n%s", r, debug.Stack()))
		}
	}()
	return handler(ctx, d)
}

// delay 返回第 attempt 次失败后的退避时长。
func (p RetryPolicy) delay(attempt int) time.Duration {
	delay := p.InitialDelay
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}

	for i := 1; i < attempt; i++ {
		delay *= 2
		if p.MaxDelay > 0 && delay >= p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		delay = p.MaxDelay
	}

	if p.Jitter > 0 {
		jitter := p.Jitter
		if jitter > 1 {
			jitter = 1
		}
		delta := float64(delay) * jitter
		delay = time.Duration(float64(delay) - delta + 2*delta*rand.Float64())
	}
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	if delay < 0 {
		delay = 0
	}
	return delay
}

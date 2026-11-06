package mqx

import (
	"errors"
	"fmt"
	"time"
)

// SubscriptionSpec 描述一次订阅。后端专属调优在实现包的 Config 中。
type SubscriptionSpec struct {
	// Destination 是 Rabbit exchange / Kafka topic / Redis stream。
	Destination string
	// BindingKey 是 AMQP topic 模式（如 user.#）：Rabbit 用于服务端绑定，
	// Kafka/Redis 用于客户端过滤。留空表示接收该 Destination 的全部消息。
	BindingKey string
	// Group 是 Rabbit queue / Kafka group / Redis group，必填。
	// 同一 Destination 下，同组订阅相互竞争，不同组各自收到一份。
	Group string
	// Concurrency 是 handler 并行度上限；实际并行度可能受后端限制
	// （Kafka 受 partition 数限制，Redis 受组内 consumer 数限制）。
	Concurrency int
	// MaxInFlight 是订阅级未确认消息上限；0 表示等于 Concurrency。
	MaxInFlight int
	// HandlerTimeout 是单次 handler 的处理时限；0 表示不额外限制。
	HandlerTimeout time.Duration
	Retry          RetryPolicy
	DeadLetter     DeadLetterPolicy
	// OnExhausted 决定重试耗尽后的去向，必须显式选择。
	// 零值为 ExhaustedDeadLetter，此时 DeadLetter.Enabled 必须为 true。
	OnExhausted ExhaustedAction
}

// RetryPolicy 描述进程内重试。零值表示不额外重试（仅处理 1 次）。
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxElapsed   time.Duration
	Jitter       float64 // 0 到 1，按延迟的百分比抖动
}

// DeadLetterPolicy 描述最终失败后的死信处理。
//
// Destination 是死信目的地（Rabbit exchange / Kafka topic / Redis stream），
// RoutingKey 是死信 routing key（为空时沿用原消息的 routing key）。
type DeadLetterPolicy struct {
	Enabled     bool
	Destination string
	RoutingKey  string
	IncludeMeta bool
}

// ExhaustedAction 是重试耗尽后的去向。
type ExhaustedAction int

const (
	// ExhaustedDeadLetter 投递到死信，要求 DeadLetter.Enabled。
	ExhaustedDeadLetter ExhaustedAction = iota
	// ExhaustedDiscard 显式丢弃，仅用于可丢失的观测类消息。
	ExhaustedDiscard
)

func (a ExhaustedAction) String() string {
	switch a {
	case ExhaustedDeadLetter:
		return "dead-letter"
	case ExhaustedDiscard:
		return "discard"
	default:
		return fmt.Sprintf("unknown(%d)", int(a))
	}
}

// Normalized 返回补全默认值后的副本。
func (s SubscriptionSpec) Normalized() SubscriptionSpec {
	if s.Concurrency == 0 {
		s.Concurrency = 1
	}
	if s.MaxInFlight == 0 {
		s.MaxInFlight = s.Concurrency
	}
	if s.MaxInFlight > 0 && s.MaxInFlight < s.Concurrency {
		s.MaxInFlight = s.Concurrency
	}
	if s.Retry.MaxAttempts == 0 {
		s.Retry.MaxAttempts = 1
	}
	return s
}

// Validate 校验必填项与语义约束。应在订阅建立前调用，让配置错误在启动时暴露。
func (s SubscriptionSpec) Validate() error {
	if s.Destination == "" {
		return fmt.Errorf("%w: destination is required", ErrInvalidSpec)
	}
	if s.Group == "" {
		return fmt.Errorf("%w: group is required", ErrInvalidSpec)
	}
	if s.Concurrency < 0 {
		return fmt.Errorf("%w: concurrency must not be negative", ErrInvalidSpec)
	}
	if s.MaxInFlight < 0 {
		return fmt.Errorf("%w: max in flight must not be negative", ErrInvalidSpec)
	}
	if s.Concurrency > 0 && s.MaxInFlight > 0 && s.MaxInFlight < s.Concurrency {
		return fmt.Errorf("%w: max in flight must not be less than concurrency", ErrInvalidSpec)
	}
	if s.HandlerTimeout < 0 {
		return fmt.Errorf("%w: handler timeout must not be negative", ErrInvalidSpec)
	}
	if s.Retry.MaxAttempts < 0 {
		return fmt.Errorf("%w: retry max attempts must not be negative", ErrInvalidSpec)
	}
	if s.Retry.InitialDelay < 0 || s.Retry.MaxDelay < 0 || s.Retry.MaxElapsed < 0 {
		return fmt.Errorf("%w: retry durations must not be negative", ErrInvalidSpec)
	}
	if s.Retry.Jitter < 0 || s.Retry.Jitter > 1 {
		return fmt.Errorf("%w: retry jitter must be within [0,1]", ErrInvalidSpec)
	}
	switch s.OnExhausted {
	case ExhaustedDeadLetter:
		if !s.DeadLetter.Enabled {
			return fmt.Errorf("%w: exhausted action is dead-letter but dead letter is disabled", ErrInvalidSpec)
		}
		if s.DeadLetter.Destination == "" {
			return fmt.Errorf("%w: dead letter destination is required", ErrInvalidSpec)
		}
		if s.DeadLetter.Destination == s.Destination {
			return fmt.Errorf("%w: dead letter destination must differ from destination", ErrInvalidSpec)
		}
	case ExhaustedDiscard:
	default:
		return fmt.Errorf("%w: unknown exhausted action %d", ErrInvalidSpec, s.OnExhausted)
	}
	return nil
}

// Prepare 返回补全默认值并校验后的规格。
func (s SubscriptionSpec) Prepare() (SubscriptionSpec, error) {
	s = s.Normalized()
	if err := s.Validate(); err != nil {
		return SubscriptionSpec{}, err
	}
	return s, nil
}

// ErrInvalidSpec 的便捷判断。
func IsInvalidSpec(err error) bool { return errors.Is(err, ErrInvalidSpec) }

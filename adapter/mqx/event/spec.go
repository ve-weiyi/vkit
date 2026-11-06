// Package event 在 mqx 之上提供类型化事件编排。
//
// 它只依赖 mqx.Broker/Publisher/Subscriber，不感知具体后端；事件类型、
// Destination 与 RoutingKey 在声明时绑定。
package event

// EventSpec 绑定事件类型、目的地与 routing key。
type EventSpec[T any] struct {
	destination string
	routingKey  string
	version     string
	producer    string
	codec       Codec
}

type eventConfig struct {
	version  string
	producer string
	codec    Codec
}

// EventOption 配置事件声明的元数据。
type EventOption func(*eventConfig)

// WithVersion 设置事件的契约版本，写入 mqx.schema_version。
func WithVersion(version string) EventOption {
	return func(c *eventConfig) { c.version = version }
}

// WithProducer 设置生产者标识，写入 mqx.producer。
func WithProducer(producer string) EventOption {
	return func(c *eventConfig) { c.producer = producer }
}

// WithCodec 替换默认的 JSON 编解码器。
func WithCodec(codec Codec) EventOption {
	return func(c *eventConfig) {
		if codec != nil {
			c.codec = codec
		}
	}
}

// NewEventSpec 声明事件类型与目的地/routing key 的绑定。
func NewEventSpec[T any](destination, routingKey string, opts ...EventOption) EventSpec[T] {
	cfg := eventConfig{codec: JSON}
	for _, opt := range opts {
		opt(&cfg)
	}
	return EventSpec[T]{
		destination: destination,
		routingKey:  routingKey,
		version:     cfg.version,
		producer:    cfg.producer,
		codec:       cfg.codec,
	}
}

// Destination 返回事件目的地（exchange/topic/stream）。
func (e EventSpec[T]) Destination() string { return e.destination }

// RoutingKey 返回事件类型（如 user.login）。
func (e EventSpec[T]) RoutingKey() string { return e.routingKey }

// Version 返回事件契约版本。
func (e EventSpec[T]) Version() string { return e.version }

// Codec 返回事件编解码器。
func (e EventSpec[T]) Codec() Codec { return e.codec }

package mqx

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message 是跨后端统一的消息模型。库不碰序列化。
//
// Message 是纯数据：确认能力在 Delivery 上，业务代码不应持有确认函数。
type Message struct {
	// ID 是消息唯一标识；为空时由适配器生成，并通过 PublishResult 返回。
	// 消费侧可把它当作幂等键。
	ID string
	// Destination 是基础设施目的地：Rabbit exchange / Kafka topic / Redis stream。
	Destination string
	// RoutingKey 是事件类型，如 user.login。Rabbit 直接作为 routing key，
	// Kafka 写入 mqx.event_type header，Redis 写入固定字段。
	RoutingKey string
	// Key 是分区/排序/业务键；Rabbit 可为空。
	Key       string
	Headers   map[string]string
	Body      []byte
	Timestamp time.Time
}

// Clone 返回深拷贝，避免发布方与消费方共享可变数据。
func (m Message) Clone() Message {
	cloned := m
	cloned.Body = append([]byte(nil), m.Body...)
	if m.Headers != nil {
		cloned.Headers = make(map[string]string, len(m.Headers))
		for k, v := range m.Headers {
			cloned.Headers[k] = v
		}
	}
	return cloned
}

// Prepared 返回补全 ID 与时间戳后的副本，并校验必填项。
func (m Message) Prepared() (Message, error) {
	out := m.Clone()
	if out.Destination == "" {
		return Message{}, fmt.Errorf("%w: destination is required", ErrBadMessage)
	}
	if out.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return Message{}, fmt.Errorf("mqx: generate message id: %w", err)
		}
		out.ID = id.String()
	}
	if out.Timestamp.IsZero() {
		out.Timestamp = time.Now()
	}
	return out, nil
}

// PublishResult 是发布结果。不同后端可用的字段不同，零值表示该字段不可用。
type PublishResult struct {
	ID        string
	Partition int32
	Offset    int64
}

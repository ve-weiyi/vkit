package mqx

import (
	"time"
)

// Message 消息体。库不碰序列化，调用方自行 marshal/unmarshal。
type Message struct {
	ID        string            // 消息唯一标识（生产者可设幂等键，空则后端自动生成）
	Key       string            // 路由键/分区键
	Body      []byte            // 消息体
	Headers   map[string]string // 业务元数据
	Timestamp time.Time         // 消息产生时间（零值时 Publisher 自动填充）

	// 确认机制。仅当 SubscribeConfig.AutoAck=false 时由 Subscriber 注入。
	// Publisher 发送的消息中这两个字段始终为 nil。业务代码不应直接设置。
	AckFn  func() error
	NackFn func(requeue bool) error
}

// Ack 手动确认消息处理成功。
// 仅在 AutoAck=false 时有效；AutoAck=true 时 AckFn 为 nil，调用无副作用。
// 多次调用安全：仅首次生效，且与 Nack 互斥（先到先得）。
func (m *Message) Ack() error {
	if m.AckFn != nil {
		return m.AckFn()
	}
	return nil
}

// Nack 手动拒绝消息。
// requeue=true:  重新入队等待重试
// requeue=false: 丢弃消息（若配置了 DLQTopic 则进入死信队列）
// 仅在 AutoAck=false 时有效；与 Ack 互斥——一旦调了 Ack，Nack 即 no-op，反之亦然。
func (m *Message) Nack(requeue bool) error {
	if m.NackFn != nil {
		return m.NackFn(requeue)
	}
	return nil
}

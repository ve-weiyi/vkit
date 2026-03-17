package mqx

import "time"

// Message 统一消息模型（适配所有消息队列）
type Message struct {
	// 基础字段
	ID        string                 // 消息ID
	Topic     string                 // 主题/队列名称
	Key       string                 // 消息键（Kafka分区键/RabbitMQ路由键）
	Body      []byte                 // 消息体
	Headers   map[string]interface{} // 消息头
	Timestamp time.Time              // 消息时间戳

	// 元数据
	Partition int32 // 分区（Kafka专用）
	Offset    int64 // 偏移量（Kafka专用）
	Attempts  int   // 重试次数

	// 内部字段（不同MQ的原始消息）
	raw interface{} // 原始消息对象

	// 确认机制
	ack  func() error
	nack func(requeue bool) error
}

// Ack 确认消息（成功处理）
func (m *Message) Ack() error {
	if m.ack != nil {
		return m.ack()
	}
	return nil
}

// Nack 拒绝消息（处理失败）
func (m *Message) Nack(requeue bool) error {
	if m.nack != nil {
		return m.nack(requeue)
	}
	return nil
}

// GetRaw 获取原始消息（类型断言使用）
func (m *Message) GetRaw() interface{} {
	return m.raw
}

// SetRaw 设置原始消息（供适配器使用）
func (m *Message) SetRaw(raw interface{}) {
	m.raw = raw
}

// SetAckFunc 设置Ack函数（供适配器使用）
func (m *Message) SetAckFunc(ack func() error) {
	m.ack = ack
}

// SetNackFunc 设置Nack函数（供适配器使用）
func (m *Message) SetNackFunc(nack func(requeue bool) error) {
	m.nack = nack
}

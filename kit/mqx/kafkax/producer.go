package kafkax

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// producer Kafka生产者实现
type producer struct {
	producer sarama.SyncProducer
	config   *Config
}

// newProducer 创建生产者
func newProducer(client sarama.Client, config *Config) (*producer, error) {
	syncProducer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync producer: %w", err)
	}

	return &producer{
		producer: syncProducer,
		config:   config,
	}, nil
}

// Send 发送单条消息
func (p *producer) Send(ctx context.Context, message *mqx.Message) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// 构建Kafka消息
	producerMessage := &sarama.ProducerMessage{
		Topic:     message.Topic,
		Key:       sarama.StringEncoder(message.Key),
		Value:     sarama.ByteEncoder(message.Body),
		Timestamp: message.Timestamp,
	}

	// 转换Headers
	if message.Headers != nil {
		headers := make([]sarama.RecordHeader, 0, len(message.Headers))
		for k, v := range message.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(fmt.Sprintf("%v", v)),
			})
		}
		producerMessage.Headers = headers
	}

	// 发送消息
	partition, offset, err := p.producer.SendMessage(producerMessage)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 更新消息的分区和偏移量信息
	message.Partition = partition
	message.Offset = offset

	return nil
}

// SendBatch 批量发送消息
func (p *producer) SendBatch(ctx context.Context, messages []*mqx.Message) error {
	if len(messages) == 0 {
		return nil
	}

	// 转换为Kafka消息
	producerMessages := make([]*sarama.ProducerMessage, 0, len(messages))
	for _, message := range messages {
		producerMessage := &sarama.ProducerMessage{
			Topic:     message.Topic,
			Key:       sarama.StringEncoder(message.Key),
			Value:     sarama.ByteEncoder(message.Body),
			Timestamp: message.Timestamp,
		}

		// 转换Headers
		if message.Headers != nil {
			headers := make([]sarama.RecordHeader, 0, len(message.Headers))
			for k, v := range message.Headers {
				headers = append(headers, sarama.RecordHeader{
					Key:   []byte(k),
					Value: []byte(fmt.Sprintf("%v", v)),
				})
			}
			producerMessage.Headers = headers
		}

		producerMessages = append(producerMessages, producerMessage)
	}

	// 批量发送
	err := p.producer.SendMessages(producerMessages)
	if err != nil {
		return fmt.Errorf("failed to send batch messages: %w", err)
	}

	return nil
}

// Close 关闭生产者
func (p *producer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

// 确保实现了接口
var _ mqx.Producer = (*producer)(nil)

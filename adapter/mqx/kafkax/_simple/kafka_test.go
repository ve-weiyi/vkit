package _simple

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// Kafka 原始示例
// 演示如何使用 sarama 库直接操作 Kafka，不使用任何封装
// 用于理解 Kafka 的基础工作原理

var brokers = []string{"localhost:9092"}

func Test_KafkaSend(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	for i := 0; i < 5; i++ {
		msg := &sarama.ProducerMessage{
			Topic: "test-topic",
			Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
			Value: sarama.StringEncoder(fmt.Sprintf("Hello Kafka! %d", i)),
		}

		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("发送失败: %v", err)
		} else {
			log.Printf("发送成功: partition=%d, offset=%d", partition, offset)
		}
		time.Sleep(1 * time.Second)
	}
}

func Test_KafkaConsume(t *testing.T) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(brokers, "test-group", config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	handler := &simpleHandler{}

	go func() {
		for {
			err := client.Consume(context.Background(), []string{"test-topic"}, handler)
			if err != nil {
				log.Printf("消费错误: %v", err)
			}
		}
	}()

	time.Sleep(30 * time.Second)
}

type simpleHandler struct{}

func (h *simpleHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *simpleHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (h *simpleHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("收到消息: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))
		sess.MarkMessage(msg, "")
	}
	return nil
}

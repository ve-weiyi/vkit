package _examples

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/redisstreamx"
)

// Redis Stream 分布式通知示例
//
// 场景：多台服务器通过 Redis Stream 实现消息广播
// 每台服务器作为消费组中的一个消费者，独立消费所有消息

func Test_RedisStreamNotify(t *testing.T) {
	mq, err := redisstreamx.NewRedisStream(&redisstreamx.Config{
		Addr: "localhost:6379",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	// 生产者：发送通知消息
	producer, err := mq.Producer()
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	err = producer.Send(context.Background(), &mqx.Message{
		ID:        "notify-001",
		Topic:     "server.notifications",
		Body:      []byte(`{"event":"config_updated","timestamp":"2024-01-01T00:00:00Z"}`),
		Timestamp: time.Now(),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("通知消息发送成功")

	// 消费者A：模拟服务器A收到通知
	mq2, err := redisstreamx.NewRedisStream(&redisstreamx.Config{
		Addr: "localhost:6379",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq2.Close()

	consumerA, err := mq2.Consumer(&mqx.ConsumerConfig{
		GroupID:       "server-group",
		Topics:        []string{"server.notifications"},
		ConsumerName:  "server-A",
		PrefetchCount: 10,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumerA.Close()

	if err := consumerA.Subscribe("server.notifications"); err != nil {
		log.Fatal(err)
	}

	received := make(chan string, 1)
	if err := consumerA.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		log.Printf("服务器A收到通知: %s", string(msg.Body))
		received <- string(msg.Body)
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	select {
	case body := <-received:
		log.Printf("处理完成: %s", body)
	case <-time.After(10 * time.Second):
		t.Fatal("等待超时")
	}
}

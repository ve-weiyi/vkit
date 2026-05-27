package _examples

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/rabbitmqx"
)

// 模拟一个 Fanout 模式：邮件订阅
// Fanout（发布/订阅）模式下，消息被广播到所有绑定到交换机的队列
// 发布者只需要声明交换机，订阅者需要声明自己的队列并绑定到交换机

const (
	ExchangeEmail = "email_exchange"
	QueueEmail    = "email_queue"  // 相同队列名称会争抢消息
	QueueEmail2   = "email_queue2" // 另一个队列，会独立收到消息
)

func Test_Fanout_Publish(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeEmail,
		ExchangeType: rabbitmqx.ExchangeTypeFanout,
		Durable:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	producer, err := mq.Producer()
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	for i := 0; i <= 100; i++ {
		fmt.Println(i)
		err = producer.Send(context.Background(), &mqx.Message{
			Topic:     ExchangeEmail,
			Body:      []byte("user email: " + strconv.Itoa(i)),
			Timestamp: time.Now(),
		})
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func Test_Fanout_Subscribe1(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeEmail,
		ExchangeType: rabbitmqx.ExchangeTypeFanout,
		Durable:      true,
		QueueConfig: &rabbitmqx.QueueConfig{
			QueueName: QueueEmail,
			Durable:   true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:       []string{QueueEmail},
		ConsumerName: "email-consumer-1",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	if err := consumer.Subscribe(QueueEmail); err != nil {
		log.Fatal(err)
	}

	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		log.Printf("receive message: %s", string(msg.Body))
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	select {}
}

func Test_Fanout_Subscribe2(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeEmail,
		ExchangeType: rabbitmqx.ExchangeTypeFanout,
		Durable:      true,
		QueueConfig: &rabbitmqx.QueueConfig{
			QueueName: QueueEmail2,
			Durable:   true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:       []string{QueueEmail2},
		ConsumerName: "email-consumer-2",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	if err := consumer.Subscribe(QueueEmail2); err != nil {
		log.Fatal(err)
	}

	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		log.Printf("receive message: %s", string(msg.Body))
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	select {}
}

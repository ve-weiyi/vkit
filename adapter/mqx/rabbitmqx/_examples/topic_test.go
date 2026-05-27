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

// 模拟一个 Topic 模式：聊天室
// Topic（主题）模式下，消息通过 Routing Key 匹配规则路由到不同的队列
// 发布者设置具体的 routing key，订阅者用通配符（*/#）匹配
// 其中 "*" 用于匹配一个单词，"#" 用于匹配多个单词（可以是零个）

const ExchangeChat = "exchange_chat"

// 发布者的 routing key（具体的）
const TopicUserOnline = "blog.chat.room_id.username.online"
const TopicUserMsg = "blog.chat.room_id.username.msg"

// 订阅者的 routing key（通配符匹配）
const TopicAllUserOnline = "blog.chat.room_id.*.online"
const TopicAllUserMsg = "blog.chat.room_id.*.msg"

func Test_Topic_Publish(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeChat,
		ExchangeType: rabbitmqx.ExchangeTypeTopic,
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
		// 发送用户上线消息
		err = producer.Send(context.Background(), &mqx.Message{
			Topic:     ExchangeChat,
			Key:       TopicUserOnline,
			Body:      []byte("user online: " + strconv.Itoa(i)),
			Timestamp: time.Now(),
		})
		if err != nil {
			log.Fatal(err)
		}
		// 发送用户消息
		err = producer.Send(context.Background(), &mqx.Message{
			Topic:     ExchangeChat,
			Key:       TopicUserMsg,
			Body:      []byte("user msg: " + strconv.Itoa(i)),
			Timestamp: time.Now(),
		})
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func Test_Topic_SubscribeOnline(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeChat,
		ExchangeType: rabbitmqx.ExchangeTypeTopic,
		Durable:      true,
		QueueConfig: &rabbitmqx.QueueConfig{
			QueueName: "online_queue",
			Durable:   true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:       []string{TopicAllUserOnline},
		ConsumerName: "online-consumer",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	// 订阅用通配符匹配所有用户的上线消息
	if err := consumer.Subscribe(TopicAllUserOnline); err != nil {
		log.Fatal(err)
	}

	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		log.Printf("receive online message: %s", string(msg.Body))
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	select {}
}

func Test_Topic_SubscribeMsg(t *testing.T) {
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: ExchangeChat,
		ExchangeType: rabbitmqx.ExchangeTypeTopic,
		Durable:      true,
		QueueConfig: &rabbitmqx.QueueConfig{
			QueueName: "msg_queue",
			Durable:   true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mq.Close()

	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:       []string{TopicAllUserMsg},
		ConsumerName: "msg-consumer",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	// 订阅用通配符匹配所有用户的聊天消息
	if err := consumer.Subscribe(TopicAllUserMsg); err != nil {
		log.Fatal(err)
	}

	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		log.Printf("receive chat message: %s", string(msg.Body))
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	select {}
}

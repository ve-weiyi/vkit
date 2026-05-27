package _simple

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ Simple 模式原始示例
// 演示如何使用 amqp 库直接操作 RabbitMQ，不使用任何封装
// 用于理解 RabbitMQ 的基础工作原理

const MQURL = "amqp://guest:guest@localhost:5672/"

func Test_SimplePublish(t *testing.T) {
	conn, err := amqp.Dial(MQURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()

	// 声明队列（不存在则创建）
	q, err := ch.QueueDeclare(
		"simple_queue", // 队列名称
		true,           // 持久化
		false,          // 自动删除
		false,          // 排他性
		false,          // 不阻塞
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// 发布消息到队列
	err = ch.PublishWithContext(
		context.Background(),
		"",       // 使用默认交换机
		q.Name,   // routing key = 队列名
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("Hello RabbitMQ Simple!"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("消息发送成功！")
}

func Test_SimpleConsume(t *testing.T) {
	conn, err := amqp.Dial(MQURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()

	// 声明队列
	q, err := ch.QueueDeclare(
		"simple_queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// 消费消息
	msgs, err := ch.Consume(
		q.Name,  // 队列名
		"",      // 消费者标签
		true,    // auto-ack
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for d := range msgs {
			log.Printf("收到消息: %s", d.Body)
		}
	}()

	log.Printf("等待消息中...")
	time.Sleep(10 * time.Second)
}

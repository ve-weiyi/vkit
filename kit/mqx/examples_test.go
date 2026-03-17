package mqx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ve-weiyi/pkg/kit/mqx"
	"github.com/ve-weiyi/pkg/kit/mqx/kafkax"
	"github.com/ve-weiyi/pkg/kit/mqx/rabbitmqx"
	"github.com/ve-weiyi/pkg/kit/mqx/redisstreamx"
)

var testKafkaConfig = &kafkax.Config{
	// 通过 hostPort 访问 k8s 集群中的 kafka-0
	Brokers: []string{"106.54.37.113:9094"},
	Version: "3.8.1",
}

const testTopic = "sparkinai-test"

func newTestKafka(t *testing.T) *kafkax.Kafka {
	t.Helper()
	mq, err := kafkax.NewKafka(testKafkaConfig)
	if err != nil {
		t.Fatalf("NewKafka: %v", err)
	}
	t.Cleanup(func() { mq.Close() })
	return mq
}

func TestKafkaPing(t *testing.T) {
	mq := newTestKafka(t)
	if err := mq.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestKafkaProducerSend(t *testing.T) {
	mq := newTestKafka(t)

	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	msg := &mqx.Message{
		ID:        "test-001",
		Topic:     testTopic,
		Key:       "test-key",
		Body:      []byte(`{"hello":"world"}`),
		Timestamp: time.Now(),
	}

	if err := producer.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	t.Logf("sent to partition=%d offset=%d", msg.Partition, msg.Offset)
}

func TestKafkaProducerSendBatch(t *testing.T) {
	mq := newTestKafka(t)

	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	msgs := make([]*mqx.Message, 3)
	for i := range msgs {
		msgs[i] = &mqx.Message{
			Topic:     testTopic,
			Key:       fmt.Sprintf("key-%d", i),
			Body:      []byte(fmt.Sprintf(`{"index":%d}`, i)),
			Timestamp: time.Now(),
		}
	}

	if err := producer.SendBatch(context.Background(), msgs); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
}

func TestKafkaConsume(t *testing.T) {
	mq := newTestKafka(t)

	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	if err := producer.Send(context.Background(), &mqx.Message{
		Topic:     testTopic,
		Key:       "consume-test",
		Body:      []byte(`{"event":"consume_test"}`),
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		GroupID:     "sparkinai-test-group",
		Topics:      []string{testTopic},
		StartOffset: mqx.OffsetEarliest,
	})
	if err != nil {
		t.Fatalf("Consumer: %v", err)
	}
	defer consumer.Close()

	if err := consumer.Subscribe(testTopic); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	received := make(chan string, 1)
	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		select {
		case received <- string(msg.Body):
		default:
		}
		return nil
	})); err != nil {
		t.Fatalf("ConsumeWithHandler: %v", err)
	}

	select {
	case body := <-received:
		t.Logf("received: %s", body)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// 示例1：使用 RabbitMQ
func TestRabbitMQ(t *testing.T) {
	// 1. 创建 RabbitMQ 实例
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: "payment.exchange",
		ExchangeType: "direct",
		Durable:      true,
		AutoDelete:   false,
		// 可选：自定义队列配置
		QueueConfig: &rabbitmqx.QueueConfig{
			QueueName:  "payment.success.queue", // 自定义队列名称
			Durable:    true,                    // 队列持久化
			AutoDelete: false,                   // 不自动删除
			Exclusive:  false,                   // 非独占
			Args: map[string]interface{}{
				"x-message-ttl": 86400000, // 消息TTL: 24小时
			},
		},
	})
	if err != nil {
		t.Fatalf("NewRabbitMQ: %v", err)
	}
	defer mq.Close()

	// 2. 创建生产者并发送消息
	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	message := &mqx.Message{
		ID:        "msg-001",
		Topic:     "payment.success",
		Key:       "payment.success",
		Body:      []byte(`{"order_no":"PAY123","amount":100.00}`),
		Timestamp: time.Now(),
		Headers: map[string]interface{}{
			"source": "payment-service",
		},
	}
	if err := producer.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}
	t.Log("message sent successfully")

	// 3. 创建消费者
	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:        []string{"payment.success"},
		ConsumerName:  "payment-consumer",
		PrefetchCount: 10,
		MaxRetries:    3,
	})
	if err != nil {
		t.Fatalf("Consumer: %v", err)
	}
	defer consumer.Close()

	// 4. 订阅主题
	if err := consumer.Subscribe("payment.success"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 5. 使用处理器消费消息
	received := make(chan string, 1)
	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		t.Logf("Received message: %s", string(msg.Body))
		// 处理业务逻辑
		select {
		case received <- string(msg.Body):
		default:
		}
		return nil
	})); err != nil {
		t.Fatalf("ConsumeWithHandler: %v", err)
	}

	select {
	case body := <-received:
		t.Logf("received: %s", body)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// 示例2：使用 Kafka
func TestKafkaProducerAndConsumer(t *testing.T) {
	// 1. 创建 Kafka 实例
	mq, err := kafkax.NewKafka(testKafkaConfig)
	if err != nil {
		t.Fatalf("NewKafka: %v", err)
	}
	defer mq.Close()

	// 2. 创建生产者并发送消息
	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	message := &mqx.Message{
		ID:        "msg-001",
		Topic:     "payment.success",
		Key:       "user-123",
		Body:      []byte(`{"order_no":"PAY123","amount":100.00}`),
		Timestamp: time.Now(),
	}
	if err := producer.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}
	t.Logf("sent to partition=%d offset=%d", message.Partition, message.Offset)

	// 3. 创建消费者
	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		GroupID:      "payment-consumer-group",
		Topics:       []string{"payment.success"},
		ConsumerName: "payment-consumer",
		AutoCommit:   false,
		StartOffset:  mqx.OffsetEarliest,
		Concurrency:  5,
	})
	if err != nil {
		t.Fatalf("Consumer: %v", err)
	}
	defer consumer.Close()

	// 4. 订阅主题
	if err := consumer.Subscribe("payment.success"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 5. 使用处理器消费消息
	received := make(chan string, 1)
	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		t.Logf("Received message from partition %d, offset %d: %s", msg.Partition, msg.Offset, string(msg.Body))
		select {
		case received <- string(msg.Body):
		default:
		}
		return nil
	})); err != nil {
		t.Fatalf("ConsumeWithHandler: %v", err)
	}

	select {
	case <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// 示例3：使用 Redis Stream
func TestRedisStream(t *testing.T) {
	// 1. 创建 Redis Stream 实例
	mq, err := redisstreamx.NewRedisStream(&redisstreamx.Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Fatalf("NewRedisStream: %v", err)
	}
	defer mq.Close()

	// 2. 创建生产者并发送消息
	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	message := &mqx.Message{
		ID:        "msg-001",
		Topic:     "payment.success",
		Key:       "user-123",
		Body:      []byte(`{"order_no":"PAY123","amount":100.00}`),
		Timestamp: time.Now(),
	}
	if err := producer.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}
	t.Log("message sent to Redis Stream")

	// 3. 创建消费者
	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		GroupID:       "payment-consumer-group",
		Topics:        []string{"payment.success"},
		ConsumerName:  "payment-consumer",
		PrefetchCount: 10,
	})
	if err != nil {
		t.Fatalf("Consumer: %v", err)
	}
	defer consumer.Close()

	// 4. 订阅主题
	if err := consumer.Subscribe("payment.success"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 5. 使用处理器消费消息
	received := make(chan string, 1)
	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		t.Logf("Received message: %s", string(msg.Body))
		select {
		case received <- string(msg.Body):
		default:
		}
		return nil
	})); err != nil {
		t.Fatalf("ConsumeWithHandler: %v", err)
	}

	select {
	case body := <-received:
		t.Logf("received: %s", body)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// 示例4：切换消息队列（零代码改动）
func TestSwitchMQ(t *testing.T) {
	// 根据配置选择不同的消息队列
	mqType := "kafka" // 可以是 "rabbitmq", "kafka", "redis"

	var mq mqx.MessageQueue
	var err error

	switch mqType {
	case "rabbitmq":
		mq, err = rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
			URL:          "amqp://guest:guest@localhost:5672/",
			ExchangeName: "payment.exchange",
			ExchangeType: "direct",
			Durable:      true,
		})
	case "kafka":
		mq, err = kafkax.NewKafka(&kafkax.Config{
			Brokers: []string{"localhost:9092"},
			Version: "3.8.1",
		})
	case "redis":
		mq, err = redisstreamx.NewRedisStream(&redisstreamx.Config{
			Addr: "localhost:6379",
		})
	default:
		t.Fatalf("unsupported mq type: %s", mqType)
	}
	if err != nil {
		t.Fatalf("init mq: %v", err)
	}
	defer mq.Close()

	// 后续代码完全相同，无需修改
	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	message := &mqx.Message{
		Topic:     "payment.success",
		Body:      []byte(`{"order_no":"PAY123"}`),
		Timestamp: time.Now(),
	}
	if err := producer.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// 示例5：实际业务场景 - 支付成功消费者
type PaymentSuccessMessage struct {
	OrderNo string  `json:"order_no"`
	UserID  string  `json:"user_id"`
	Amount  float64 `json:"amount"`
}

func TestPaymentConsumer(t *testing.T) {
	// 创建消息队列（以RabbitMQ为例）
	mq, err := rabbitmqx.NewRabbitMQ(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: "payment.exchange",
		ExchangeType: "direct",
		Durable:      true,
	})
	if err != nil {
		t.Fatalf("NewRabbitMQ: %v", err)
	}
	defer mq.Close()

	producer, err := mq.Producer()
	if err != nil {
		t.Fatalf("Producer: %v", err)
	}
	defer producer.Close()

	payload, _ := json.Marshal(PaymentSuccessMessage{OrderNo: "PAY123", UserID: "user-1", Amount: 100.00})
	if err := producer.Send(context.Background(), &mqx.Message{
		Topic:     "payment.success",
		Body:      payload,
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// 创建消费者
	consumer, err := mq.Consumer(&mqx.ConsumerConfig{
		Topics:        []string{"payment.success"},
		ConsumerName:  "payment-success-consumer",
		PrefetchCount: 10,
		MaxRetries:    3,
		EnableDLQ:     true,
		DLQTopic:      "payment.dlq",
	})
	if err != nil {
		t.Fatalf("Consumer: %v", err)
	}
	defer consumer.Close()

	// 订阅主题
	if err := consumer.Subscribe("payment.success"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 定义消息处理器
	received := make(chan PaymentSuccessMessage, 1)
	if err := consumer.ConsumeWithHandler(mqx.MessageHandlerFunc(func(ctx context.Context, msg *mqx.Message) error {
		// 1. 解析消息
		var paymentMsg PaymentSuccessMessage
		if err := json.Unmarshal(msg.Body, &paymentMsg); err != nil {
			return fmt.Errorf("failed to unmarshal message: %w", err)
		}

		t.Logf("Processing payment: OrderNo=%s, UserID=%s, Amount=%.2f",
			paymentMsg.OrderNo, paymentMsg.UserID, paymentMsg.Amount)

		// 2. 调用业务逻辑（例如：为用户充值）
		// err := adjustUserBalance(ctx, paymentMsg.UserID, paymentMsg.Amount)
		// if err != nil {
		//     return err
		// }

		// 3. 发送通知
		// sendNotification(ctx, paymentMsg.UserID, "充值成功")

		select {
		case received <- paymentMsg:
		default:
		}
		return nil
	})); err != nil {
		t.Fatalf("ConsumeWithHandler: %v", err)
	}

	select {
	case p := <-received:
		t.Logf("processed payment: OrderNo=%s UserID=%s Amount=%.2f", p.OrderNo, p.UserID, p.Amount)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

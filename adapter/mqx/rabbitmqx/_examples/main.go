// Package main 演示 rabbitmqx 的基本使用方式。
// 运行前需启动 RabbitMQ 实例（如 docker run -d --name rabbitmq -p 5672:5672 rabbitmq:3-management）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/rabbitmqx"
)

func main() {
	// ── 1. 创建 MessageQueue ──
	mq, err := rabbitmqx.New(&rabbitmqx.Config{
		URL:          "amqp://guest:guest@localhost:5672/",
		ExchangeName: "orders",
		ExchangeType: rabbitmqx.ExchangeTopic,
		Durable:      true,
		Reconnect: rabbitmqx.ReconnectConfig{
			MaxRetries:   0, // 0 = 无限重连
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create message queue: %v\n", err)
		os.Exit(1)
	}
	defer mq.Close()

	// ── 2. Ping 检查连通性 ──
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := mq.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "broker unreachable: %v\n", err)
		cancel()
		os.Exit(1)
	}
	cancel()

	// ── 3. 启动消费者 ──
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sub, err := mq.Subscriber()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create subscriber: %v\n", err)
			return
		}
		defer sub.Close()

		// 用中间件包装 handler
		handler := mqx.Chain(
			mqx.Recovery(nil), // panic 恢复
			mqx.Logging(&stdLogger{}),
			mqx.Retry(3),
		)(processOrder)

		fmt.Println("[subscriber] waiting for orders...")
		if err := sub.Subscribe(context.Background(),
			"order.created",
			handler,
			mqx.WithGroup("order-service"),
			mqx.WithConcurrency(4),
			mqx.WithPrefetch(10),
		); err != nil {
			fmt.Fprintf(os.Stderr, "subscribe error: %v\n", err)
		}
	}()

	// ── 4. 发布消息 ──
	pub, err := mq.Publisher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create publisher: %v\n", err)
		os.Exit(1)
	}
	defer pub.Close()

	order := map[string]interface{}{
		"order_id": "ORD-20240001",
		"amount":   99.99,
		"currency": "CNY",
	}
	body, _ := json.Marshal(order)

	for i := 0; i < 3; i++ {
		msg := &mqx.Message{
			ID:        fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), i),
			Body:      body,
			Headers:   map[string]string{"trace_id": fmt.Sprintf("trace-%d", i)},
			Timestamp: time.Now(),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := pub.Publish(ctx, "order.created", msg, mqx.WithKey("order")); err != nil {
			fmt.Fprintf(os.Stderr, "publish error: %v\n", err)
		} else {
			fmt.Printf("[publisher] sent order %d\n", i+1)
		}
		cancel()
	}

	// ── 5. 等待信号优雅退出 ──
	fmt.Println("waiting for signal...")
	<-sigCh
	fmt.Println("shutting down")
}

// processOrder 处理订单消息。
func processOrder(ctx context.Context, msg *mqx.Message) error {
	var order struct {
		OrderID  string  `json:"order_id"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := json.Unmarshal(msg.Body, &order); err != nil {
		return fmt.Errorf("unmarshal: %w", err) // 返回 error → NACK，触发重试
	}

	fmt.Printf("[handler] processing %s: %.2f %s (trace=%s)\n",
		order.OrderID, order.Amount, order.Currency, msg.Headers["trace_id"])
	return nil // 返回 nil → ACK
}

// stdLogger 简单的日志实现。
type stdLogger struct{}

func (l *stdLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, keysAndValues)
}

func (l *stdLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, keysAndValues)
}

// Package main 演示 kafkax 的基本使用方式。
// 运行前需启动 Kafka 实例（如 docker compose up -d 或本地 kafka-server-start）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/kafkax"
)

func main() {
	// ── 1. 创建 MessageQueue ──
	mq, err := kafkax.New(&kafkax.Config{
		Seeds:        []string{"localhost:9092"},
		RequiredAcks: kafkax.AllISRAcks(),        // 最安全：等所有 ISR 副本确认
		Compression:  kafkax.SnappyCompression(), // Snappy 压缩，权衡速度与体积
		MaxRetries:   3,
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

	// ── 3. 启动消费者（Consumer Group） ──
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	consumeCtx, stopConsume := context.WithCancel(context.Background())
	defer stopConsume()

	go func() {
		sub, err := mq.Subscriber()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create subscriber: %v\n", err)
			return
		}
		defer sub.Close()

		handler := mqx.Chain(
			mqx.Recovery(nil),
			mqx.Logging(&stdLogger{}),
			mqx.Retry(3),
		)(processEvent)

		fmt.Println("[subscriber] listening on topic user-events")
		if err := sub.Subscribe(consumeCtx,
			"user-events",
			handler,
			mqx.WithGroup("analytics-consumer"),
			mqx.WithConcurrency(4), // 4 个 goroutine 并发 PollFetches
			mqx.WithPrefetch(50),
			mqx.WithDLQ("user-events-dlq"),
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

	events := []struct {
		Event string `json:"event"`
		User  string `json:"user"`
		Data  string `json:"data"`
	}{
		{"page_view", "alice", "/products/123"},
		{"click", "bob", "add_to_cart"},
		{"purchase", "alice", "ORD-20240001"},
	}

	// 按 user 分区：同一用户的事件进入同一 partition，保证顺序
	for i, e := range events {
		body, _ := json.Marshal(e)

		msg := &mqx.Message{
			ID:        fmt.Sprintf("event-%d", i),
			Key:       e.User, // 分区键 = user ID
			Body:      body,
			Headers:   map[string]string{"source": "web", "version": "1.0"},
			Timestamp: time.Now(),
		}

		if err := pub.Publish(context.Background(), "user-events", msg,
			mqx.WithKey(e.User), // PublishOption 覆盖 Message.Key
			mqx.WithTimeout(5*time.Second),
		); err != nil {
			fmt.Fprintf(os.Stderr, "publish error: %v\n", err)
		} else {
			fmt.Printf("[publisher] sent %s event from %s\n", e.Event, e.User)
		}
	}

	// ── 5. 演示 per-publish Ack 覆盖 ──
	// 对非关键日志类消息使用 LeaderAck 提高吞吐
	fmt.Println("\n--- demonstrating WithKafkaAcks ---")
	metricsMsg := &mqx.Message{
		ID:   "metrics-demo",
		Body: []byte(`{"cpu": 45.2, "mem": 72.1}`),
	}
	if err := pub.Publish(context.Background(), "user-events", metricsMsg,
		kafkax.WithKafkaAcks(kafkax.LeaderAck()), // 本次发布仅等 leader 确认
	); err != nil {
		fmt.Fprintf(os.Stderr, "metrics publish error: %v\n", err)
	} else {
		fmt.Println("[publisher] sent metrics with LeaderAck (faster, slightly less safe)")
	}

	// ── 6. 等待信号 ──
	fmt.Println("\nwaiting for signal (ctrl+c to exit)...")
	<-sigCh
	fmt.Println("shutting down...")
	stopConsume()
	time.Sleep(500 * time.Millisecond)
}

func processEvent(ctx context.Context, msg *mqx.Message) error {
	var e struct {
		Event string `json:"event"`
		User  string `json:"user"`
		Data  string `json:"data"`
	}
	if err := json.Unmarshal(msg.Body, &e); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	fmt.Printf("[handler] event=%s user=%s data=%s partition=%s\n",
		e.Event, e.User, e.Data, msg.Key)
	return nil
}

type stdLogger struct{}

func (l *stdLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, sprintKV(keysAndValues))
}

func (l *stdLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, sprintKV(keysAndValues))
}

func sprintKV(kv []interface{}) string {
	var b strings.Builder
	for i, v := range kv {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%v", v)
	}
	return b.String()
}

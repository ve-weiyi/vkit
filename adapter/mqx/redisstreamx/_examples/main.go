// Package main 演示 redisstreamx 的基本使用方式。
// 运行前需启动 Redis 实例（如 docker run -d --name redis -p 6379:6379 redis:7）。
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
	"github.com/ve-weiyi/vkit/adapter/mqx/redisstreamx"
)

func main() {
	// ── 1. 创建 MessageQueue ──
	mq, err := redisstreamx.New(&redisstreamx.Config{
		Addrs: []string{"localhost:6379"},
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

	// ── 3. 启动消费者（Consumer Group 模式） ──
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 创建消费 context，可取消来停止 Subscribe
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
			mqx.Retry(3),
		)(sendNotification)

		fmt.Println("[subscriber] listening on stream notifications")
		if err := sub.Subscribe(consumeCtx,
			"notifications",
			handler,
			mqx.WithGroup("notification-service"),
			mqx.WithConcurrency(2),
			mqx.WithPrefetch(5),
			mqx.WithDLQ("notifications-dlq"), // 重试耗尽后写入死信
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

	notifications := []struct {
		UserID  string `json:"user_id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}{
		{"user-001", "订单已发货", "您的订单 ORD-001 已由顺丰发出"},
		{"user-002", "退款到账", "退款 99.99 元已退回原支付账户"},
		{"user-001", "促销提醒", "您关注的商品降至 49.99 元"},
	}

	// 单条发送
	for i, n := range notifications {
		body, _ := json.Marshal(n)

		msg := &mqx.Message{
			ID:        fmt.Sprintf("notif-%d", i),
			Body:      body,
			Headers:   map[string]string{"type": "push"},
			Timestamp: time.Now(),
		}

		if err := pub.Publish(context.Background(), "notifications", msg,
			mqx.WithTimeout(3*time.Second),
		); err != nil {
			fmt.Fprintf(os.Stderr, "publish error: %v\n", err)
		} else {
			fmt.Printf("[publisher] sent notification to %s\n", n.UserID)
		}
	}

	// 批量发送
	batch := []*mqx.Message{
		{ID: "batch-1", Body: []byte(`{"user_id":"user-003","title":"系统通知","content":"服务升级完成"}`), Headers: map[string]string{"type": "system"}, Timestamp: time.Now()},
		{ID: "batch-2", Body: []byte(`{"user_id":"user-004","title":"安全提醒","content":"检测到新设备登录"}`), Headers: map[string]string{"type": "alert"}, Timestamp: time.Now()},
	}
	if err := pub.PublishBatch(context.Background(), "notifications", batch,
		mqx.WithTimeout(5*time.Second),
	); err != nil {
		fmt.Fprintf(os.Stderr, "batch publish error: %v\n", err)
	} else {
		fmt.Printf("[publisher] batch sent %d notifications\n", len(batch))
	}

	// ── 5. 等待信号 ──
	fmt.Println("waiting for signal (ctrl+c to exit)...")
	<-sigCh
	fmt.Println("shutting down...")
	stopConsume()
	time.Sleep(500 * time.Millisecond) // 给 Subscribe 一点时间退出
}

func sendNotification(ctx context.Context, msg *mqx.Message) error {
	var n struct {
		UserID  string `json:"user_id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(msg.Body, &n); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	fmt.Printf("[handler] push to %s: [%s] %s\n", n.UserID, n.Title, n.Content)
	return nil
}

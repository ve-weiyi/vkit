//go:build integration

package mqtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/rabbitmqx"
)

// TestRabbitConformance 需要真实 RabbitMQ：
//
//	MQX_RABBIT_URL=amqp://guest:guest@localhost:5672/ go test -tags integration ./adapter/mqx/mqtest/
func TestRabbitConformance(t *testing.T) {
	url := os.Getenv("MQX_RABBIT_URL")
	if url == "" {
		t.Skip("MQX_RABBIT_URL not set")
	}
	runConformance(t, func(t *testing.T) mqx.Broker {
		broker, err := rabbitmqx.NewBroker(&rabbitmqx.Config{
			URL:         url,
			Transient:   true,
			LazyConnect: false,
		})
		if err != nil {
			t.Fatalf("NewBroker: %v", err)
		}
		return broker
	})
}

// TestRabbitUnroutablePublish 验证 mandatory + basic.return：exchange 没有绑定队列时必须返回错误。
func TestRabbitUnroutablePublish(t *testing.T) {
	url := os.Getenv("MQX_RABBIT_URL")
	if url == "" {
		t.Skip("MQX_RABBIT_URL not set")
	}
	broker, err := rabbitmqx.NewBroker(&rabbitmqx.Config{URL: url, LazyConnect: false})
	if err != nil {
		t.Fatalf("NewBroker: %v", err)
	}
	t.Cleanup(func() { _ = broker.Close(context.Background()) })

	pub, err := broker.Publisher()
	if err != nil {
		t.Fatalf("Publisher: %v", err)
	}

	destination := fmt.Sprintf("test.unroutable.%d", time.Now().UnixNano())
	t.Cleanup(func() {
		conn, err := amqp.Dial(url)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		ch, err := conn.Channel()
		if err != nil {
			return
		}
		defer func() { _ = ch.Close() }()
		_ = ch.ExchangeDelete(destination, false, false)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = pub.Publish(ctx, &mqx.Message{
		Destination: destination,
		RoutingKey:  "test.event",
		Body:        []byte("payload"),
	})
	if err == nil {
		t.Fatal("unroutable publish should fail, got nil")
	}
	if !strings.Contains(err.Error(), "unroutable") {
		t.Fatalf("want unroutable error, got %v", err)
	}
}

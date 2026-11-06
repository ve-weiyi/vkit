// Package mqtest 提供 mqx.Broker 的契约测试。
//
// 适配器只需提供一个 Broker 工厂，即可验证是否满足统一语义。
package mqtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// BrokerFactory 为每个子测试创建独立的 Broker。
type BrokerFactory func(t *testing.T) mqx.Broker

const recvTimeout = 5 * time.Second

// runConformance 运行全部契约用例。
func runConformance(t *testing.T, newBroker BrokerFactory) {
	t.Helper()

	t.Run("PublishSubscribe", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		ch := make(chan *mqx.Delivery, 4)
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest,
			Group:       "publish-subscribe",
			OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(ch))

		result, err := pub.Publish(context.Background(), &mqx.Message{
			Destination: dest,
			RoutingKey:  "user.created",
			Key:         "key-1",
			Headers:     map[string]string{"x-trace": "abc"},
			Body:        []byte("payload"),
		})
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if result == nil || result.ID == "" {
			t.Fatalf("PublishResult = %+v, want non-empty ID", result)
		}

		d := recv(t, ch)
		if d.Destination != dest || d.RoutingKey != "user.created" {
			t.Fatalf("delivery = %s/%s", d.Destination, d.RoutingKey)
		}
		if d.Key != "key-1" || string(d.Body) != "payload" || d.Headers["x-trace"] != "abc" {
			t.Fatalf("delivery payload = %+v", d)
		}
		if d.ID == "" || d.ID != result.ID {
			t.Fatalf("delivery ID = %q, result ID = %q", d.ID, result.ID)
		}
		if d.Timestamp.IsZero() {
			t.Fatal("Timestamp 未填充")
		}
		closeSubscription(t, sub)
	})

	t.Run("RoutingKeyFilter", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		createdCh := make(chan *mqx.Delivery, 4)
		allCh := make(chan *mqx.Delivery, 4)

		created := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, BindingKey: "user.created", Group: "g-created",
			OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(createdCh))
		all := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, BindingKey: "user.#", Group: "g-all",
			OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(allCh))

		for _, key := range []string{"user.created", "user.deleted"} {
			if _, err := pub.Publish(context.Background(), &mqx.Message{
				Destination: dest, RoutingKey: key, Body: []byte(key),
			}); err != nil {
				t.Fatalf("Publish %s: %v", key, err)
			}
		}

		if d := recv(t, createdCh); d.RoutingKey != "user.created" {
			t.Fatalf("created 订阅收到 %s", d.RoutingKey)
		}
		expectNone(t, createdCh)
		if d := recv(t, allCh); d.RoutingKey != "user.created" {
			t.Fatalf("all 订阅收到 %s", d.RoutingKey)
		}
		if d := recv(t, allCh); d.RoutingKey != "user.deleted" {
			t.Fatalf("all 订阅收到 %s", d.RoutingKey)
		}
		closeSubscription(t, created)
		closeSubscription(t, all)
	})

	t.Run("GroupFanout", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		firstCh := make(chan *mqx.Delivery, 8)
		secondCh := make(chan *mqx.Delivery, 8)

		first := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "group-a", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(firstCh))
		second := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "group-b", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(secondCh))

		publishN(t, pub, dest, 2)
		for i := 0; i < 2; i++ {
			recv(t, firstCh)
			recv(t, secondCh)
		}
		closeSubscription(t, first)
		closeSubscription(t, second)
	})

	t.Run("GroupCompetingConsumers", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)

		var first, second atomic.Int32
		handled := make(chan struct{}, 8)
		firstSub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "shared", OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			first.Add(1)
			handled <- struct{}{}
			return nil
		})
		secondSub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "shared", OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			second.Add(1)
			handled <- struct{}{}
			return nil
		})

		const total = 6
		publishN(t, pub, dest, total)
		for i := 0; i < total; i++ {
			recv(t, handled)
		}

		if got := first.Load() + second.Load(); got != total {
			t.Fatalf("同组消费总数 = %d, want %d（每条消息只投给组内一个订阅）", got, total)
		}
		closeSubscription(t, firstSub)
		closeSubscription(t, secondSub)
	})

	t.Run("ManualAckStopsRetry", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		var calls atomic.Int32
		seen := make(chan struct{}, 1)

		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "manual-ack",
			Retry:       mqx.RetryPolicy{MaxAttempts: 3, InitialDelay: time.Millisecond},
			OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			calls.Add(1)
			select {
			case seen <- struct{}{}:
			default:
			}
			_ = d.Ack()
			return errors.New("ignored after manual ack")
		})

		publishN(t, pub, dest, 1)
		recv(t, seen)
		// 手动确认后不应再收到同一消息；用最小静默窗口验证没有重试。
		expectNone(t, seen)
		if got := calls.Load(); got != 1 {
			t.Fatalf("calls = %d, want 1（手动确认后不得重试）", got)
		}
		closeSubscription(t, sub)
	})

	t.Run("HandlerTimeout", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		dlq := dest + ".dlq"
		timedOut := make(chan struct{}, 1)
		dlqCh := make(chan *mqx.Delivery, 2)

		dlqSub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dlq, Group: "timeout-dlq", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(dlqCh))
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination:    dest,
			Group:          "timeout",
			HandlerTimeout: 20 * time.Millisecond,
			Retry:          mqx.RetryPolicy{MaxAttempts: 1},
			DeadLetter:     mqx.DeadLetterPolicy{Enabled: true, Destination: dlq},
			OnExhausted:    mqx.ExhaustedDeadLetter,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			<-ctx.Done()
			timedOut <- struct{}{}
			return ctx.Err()
		})

		publishN(t, pub, dest, 1)
		recv(t, timedOut)
		recv(t, dlqCh)
		closeSubscription(t, sub)
		closeSubscription(t, dlqSub)
	})

	t.Run("ManualNackRequeues", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		var calls atomic.Int32
		done := make(chan struct{}, 1)

		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "manual-nack", OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			if calls.Add(1) == 1 {
				return d.Nack(true)
			}
			done <- struct{}{}
			return nil
		})

		publishN(t, pub, dest, 1)
		recv(t, done)
		if got := calls.Load(); got != 2 {
			t.Fatalf("calls = %d, want 2（Nack(true) 应重投一次）", got)
		}
		closeSubscription(t, sub)
	})

	t.Run("RetryThenSuccess", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		var calls atomic.Int32
		done := make(chan struct{}, 1)

		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "retry",
			Retry:       mqx.RetryPolicy{MaxAttempts: 3, InitialDelay: time.Millisecond},
			OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			if calls.Add(1) < 3 {
				return errors.New("transient")
			}
			done <- struct{}{}
			return nil
		})

		publishN(t, pub, dest, 1)
		recv(t, done)
		if got := calls.Load(); got != 3 {
			t.Fatalf("calls = %d, want 3", got)
		}
		closeSubscription(t, sub)
	})

	t.Run("PermanentNoRetry", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		dlq := dest + ".dlq"
		var calls atomic.Int32
		dlqCh := make(chan *mqx.Delivery, 2)

		dlqSub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dlq, Group: "dlq", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(dlqCh))
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "permanent",
			Retry: mqx.RetryPolicy{MaxAttempts: 5, InitialDelay: time.Millisecond},
			DeadLetter: mqx.DeadLetterPolicy{
				Enabled: true, Destination: dlq, IncludeMeta: true,
			},
			OnExhausted: mqx.ExhaustedDeadLetter,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			calls.Add(1)
			return mqx.Permanent(errors.New("poison"))
		})

		publishN(t, pub, dest, 1)
		d := recv(t, dlqCh)
		if d.Headers[mqx.HeaderOriginalDestination] != dest {
			t.Fatalf("DLQ headers = %+v", d.Headers)
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("calls = %d, want 1（Permanent 不重试）", got)
		}
		closeSubscription(t, sub)
		closeSubscription(t, dlqSub)
	})

	t.Run("PanicIsPermanent", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		dlq := dest + ".dlq"
		var calls atomic.Int32
		dlqCh := make(chan *mqx.Delivery, 2)

		dlqSub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dlq, Group: "dlq", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(dlqCh))
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "panic",
			Retry:       mqx.RetryPolicy{MaxAttempts: 3, InitialDelay: time.Millisecond},
			DeadLetter:  mqx.DeadLetterPolicy{Enabled: true, Destination: dlq},
			OnExhausted: mqx.ExhaustedDeadLetter,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			calls.Add(1)
			panic("boom")
		})

		publishN(t, pub, dest, 1)
		recv(t, dlqCh)
		if got := calls.Load(); got != 1 {
			t.Fatalf("calls = %d, want 1（panic 不重试）", got)
		}
		closeSubscription(t, sub)
		closeSubscription(t, dlqSub)
	})

	t.Run("ExhaustedDiscard", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		dest := destination(t)
		var calls atomic.Int32
		failed := make(chan struct{}, 2)

		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "discard",
			Retry:       mqx.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond},
			OnExhausted: mqx.ExhaustedDiscard,
		}, func(ctx context.Context, d *mqx.Delivery) error {
			calls.Add(1)
			failed <- struct{}{}
			return errors.New("always")
		})

		publishN(t, pub, dest, 1)
		recv(t, failed)
		recv(t, failed)
		if got := calls.Load(); got != 2 {
			t.Fatalf("calls = %d, want 2", got)
		}
		closeSubscription(t, sub)
	})

	t.Run("InvalidSpec", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		subscriber, err := broker.Subscriber()
		if err != nil {
			t.Fatalf("Subscriber: %v", err)
		}
		sub, err := subscriber.Subscribe(context.Background(), mqx.SubscriptionSpec{
			Destination: "dest", Group: "g",
		}, func(context.Context, *mqx.Delivery) error { return nil })
		if err == nil {
			_ = sub.Close(context.Background())
			t.Fatal("want invalid spec error, got nil")
		}
		if !mqx.IsInvalidSpec(err) {
			t.Fatalf("error = %v, want ErrInvalidSpec", err)
		}
	})

	t.Run("CloseIsIdempotent", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: destination(t), Group: "close", OnExhausted: mqx.ExhaustedDiscard,
		}, func(context.Context, *mqx.Delivery) error { return nil })

		closeSubscription(t, sub)
		select {
		case _, ok := <-sub.Err():
			if ok {
				t.Fatal("Err channel 应已关闭")
			}
		case <-time.After(recvTimeout):
			t.Fatal("Close 后 Err channel 未关闭")
		}
		if err := sub.Close(context.Background()); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	})

	t.Run("BatchPublish", func(t *testing.T) {
		broker := brokerOf(t, newBroker)
		pub := publisher(t, broker)
		batch, ok := pub.(mqx.BatchPublisher)
		if !ok {
			t.Skip("backend does not implement BatchPublisher")
		}
		dest := destination(t)
		ch := make(chan *mqx.Delivery, 4)
		sub := subscribe(t, broker, mqx.SubscriptionSpec{
			Destination: dest, Group: "batch", OnExhausted: mqx.ExhaustedDiscard,
		}, chanHandler(ch))

		results, err := batch.PublishBatch(context.Background(), []*mqx.Message{
			{Destination: dest, RoutingKey: "batch", Body: []byte("a")},
			{Destination: dest, RoutingKey: "batch", Body: []byte("b")},
		})
		if err != nil {
			t.Fatalf("PublishBatch: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("results = %d, want 2", len(results))
		}
		recv(t, ch)
		recv(t, ch)
		closeSubscription(t, sub)
	})
}

func brokerOf(t *testing.T, newBroker BrokerFactory) mqx.Broker {
	t.Helper()
	broker := newBroker(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), recvTimeout)
		defer cancel()
		_ = broker.Close(ctx)
	})
	return broker
}

func publisher(t *testing.T, broker mqx.Broker) mqx.Publisher {
	t.Helper()
	pub, err := broker.Publisher()
	if err != nil {
		t.Fatalf("Publisher: %v", err)
	}
	return pub
}

func subscribe(t *testing.T, broker mqx.Broker, spec mqx.SubscriptionSpec, handler mqx.Handler) mqx.Subscription {
	t.Helper()
	sub, err := broker.Subscriber()
	if err != nil {
		t.Fatalf("Subscriber: %v", err)
	}
	subscription, err := sub.Subscribe(context.Background(), spec, handler)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(func() { closeSubscription(t, subscription) })
	return subscription
}

func closeSubscription(t *testing.T, sub mqx.Subscription) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), recvTimeout)
	defer cancel()
	if err := sub.Close(ctx); err != nil {
		t.Fatalf("Close subscription: %v", err)
	}
}

func chanHandler(ch chan<- *mqx.Delivery) mqx.Handler {
	return func(ctx context.Context, d *mqx.Delivery) error {
		ch <- d
		return nil
	}
}

func publishN(t *testing.T, pub mqx.Publisher, dest string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := pub.Publish(context.Background(), &mqx.Message{
			Destination: dest, RoutingKey: "test.event", Body: []byte("payload"),
		}); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(recvTimeout):
		t.Fatal("等待消息超时")
		var zero T
		return zero
	}
}

func expectNone[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("不应收到消息: %+v", v)
	case <-time.After(100 * time.Millisecond):
	}
}

var destinationSeq atomic.Int64

func destination(t *testing.T) string {
	t.Helper()
	// Kafka 主题名不允许出现 '/'，子测试名里的分隔符换成 '.'。
	name := strings.ReplaceAll(t.Name(), "/", ".")
	return fmt.Sprintf("test.%s.%d", name, destinationSeq.Add(1))
}

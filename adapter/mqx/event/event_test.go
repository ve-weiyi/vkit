package event_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/event"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/memmqx"
)

type userCreated struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestPublishSubscribe(t *testing.T) {
	broker := memmqx.NewBroker(memmqx.Config{QueueSize: 64})
	runtime := event.NewRuntime(broker, nil)
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	spec := event.NewEventSpec[userCreated]("test.user", "user.created",
		event.WithVersion("v1"), event.WithProducer("test"))
	router, err := runtime.NewRouter()
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	received := make(chan userCreated, 1)
	if err := event.HandleEvent(router, spec, func(ctx context.Context, e *userCreated) error {
		received <- *e
		return nil
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if err := runtime.Start(router, mqx.SubscriptionSpec{
		Destination: "test.user",
		BindingKey:  "user.#",
		Group:       "event-test",
		OnExhausted: mqx.ExhaustedDiscard,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	pub, err := runtime.NewPublisher()
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	result, err := spec.Publish(context.Background(), pub, &userCreated{ID: 7, Name: "alice"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.ID == "" {
		t.Fatal("publish result ID is empty")
	}

	select {
	case got := <-received:
		if got.ID != 7 || got.Name != "alice" {
			t.Fatalf("received = %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("等待事件超时")
	}
}

func TestEnvelopeHeaders(t *testing.T) {
	broker := memmqx.NewBroker(memmqx.Config{QueueSize: 64})
	t.Cleanup(func() { _ = broker.Close(context.Background()) })

	subscriber, err := broker.Subscriber()
	if err != nil {
		t.Fatalf("Subscriber: %v", err)
	}
	deliveries := make(chan *mqx.Delivery, 1)
	sub, err := subscriber.Subscribe(context.Background(), mqx.SubscriptionSpec{
		Destination: "test.user",
		Group:       "headers",
		OnExhausted: mqx.ExhaustedDiscard,
	}, func(ctx context.Context, d *mqx.Delivery) error {
		deliveries <- d
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(func() { _ = sub.Close(context.Background()) })

	pub, err := broker.Publisher()
	if err != nil {
		t.Fatalf("Publisher: %v", err)
	}
	spec := event.NewEventSpec[userCreated]("test.user", "user.created",
		event.WithVersion("v1"), event.WithProducer("test"))
	if _, err := spec.Publish(context.Background(), pub, &userCreated{ID: 1}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	d := recv(t, deliveries)
	required := []string{
		mqx.HeaderEventID,
		mqx.HeaderEventType,
		mqx.HeaderSchemaVersion,
		mqx.HeaderOccurredAt,
		mqx.HeaderProducer,
	}
	for _, key := range required {
		if d.Headers[key] == "" {
			t.Fatalf("header %s is empty, headers = %+v", key, d.Headers)
		}
	}
	if d.Headers[mqx.HeaderEventType] != "user.created" {
		t.Fatalf("event type = %q", d.Headers[mqx.HeaderEventType])
	}
}

func TestDecodeFailureGoesToDeadLetter(t *testing.T) {
	broker := memmqx.NewBroker(memmqx.Config{QueueSize: 64})
	runtime := event.NewRuntime(broker, nil)
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	spec := event.NewEventSpec[userCreated]("test.user", "user.created")
	router, err := runtime.NewRouter()
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	var called atomic.Bool
	if err := event.HandleEvent(router, spec, func(ctx context.Context, e *userCreated) error {
		called.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	deadLetters := make(chan *mqx.Delivery, 1)
	deadSub, err := broker.Subscriber()
	if err != nil {
		t.Fatalf("Subscriber: %v", err)
	}
	dead, err := deadSub.Subscribe(context.Background(), mqx.SubscriptionSpec{
		Destination: "test.user.dlq",
		Group:       "dlq",
		OnExhausted: mqx.ExhaustedDiscard,
	}, func(ctx context.Context, d *mqx.Delivery) error {
		deadLetters <- d
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe dlq: %v", err)
	}
	t.Cleanup(func() { _ = dead.Close(context.Background()) })

	if err := runtime.Start(router, mqx.SubscriptionSpec{
		Destination: "test.user",
		Group:       "decode",
		DeadLetter:  mqx.DeadLetterPolicy{Enabled: true, Destination: "test.user.dlq", IncludeMeta: true},
		OnExhausted: mqx.ExhaustedDeadLetter,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	pub, err := broker.Publisher()
	if err != nil {
		t.Fatalf("Publisher: %v", err)
	}
	if _, err := pub.Publish(context.Background(), &mqx.Message{
		Destination: "test.user",
		RoutingKey:  "user.created",
		Body:        []byte("not-json"),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	d := recv(t, deadLetters)
	if d.Headers[mqx.HeaderOriginalRoutingKey] != "user.created" {
		t.Fatalf("dead letter headers = %+v", d.Headers)
	}
	if called.Load() {
		t.Fatal("解码失败时不应调用业务 handler")
	}
}

func TestUnhandledRoutingKeyGoesToDeadLetter(t *testing.T) {
	broker := memmqx.NewBroker(memmqx.Config{QueueSize: 64})
	runtime := event.NewRuntime(broker, nil)
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	deadLetters := make(chan *mqx.Delivery, 1)
	deadSub, err := broker.Subscriber()
	if err != nil {
		t.Fatalf("Subscriber: %v", err)
	}
	dead, err := deadSub.Subscribe(context.Background(), mqx.SubscriptionSpec{
		Destination: "test.user.dlq",
		Group:       "dlq",
		OnExhausted: mqx.ExhaustedDiscard,
	}, func(ctx context.Context, d *mqx.Delivery) error {
		deadLetters <- d
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe dlq: %v", err)
	}
	t.Cleanup(func() { _ = dead.Close(context.Background()) })

	// 不注册任何处理器：未知 routing key 必须走失败去向，而不是被 Ack。
	router, err := runtime.NewRouter()
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if err := runtime.Start(router, mqx.SubscriptionSpec{
		Destination: "test.user",
		Group:       "unhandled",
		DeadLetter:  mqx.DeadLetterPolicy{Enabled: true, Destination: "test.user.dlq", IncludeMeta: true},
		OnExhausted: mqx.ExhaustedDeadLetter,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	pub, err := broker.Publisher()
	if err != nil {
		t.Fatalf("Publisher: %v", err)
	}
	if _, err := pub.Publish(context.Background(), &mqx.Message{
		Destination: "test.user",
		RoutingKey:  "user.created",
		Body:        []byte(`{"id":1}`),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	d := recv(t, deadLetters)
	if d.Headers[mqx.HeaderOriginalRoutingKey] != "user.created" {
		t.Fatalf("dead letter headers = %+v", d.Headers)
	}
}

func TestSubscriptionRestart(t *testing.T) {
	subscriber := &failingSubscriber{}
	runtime := event.NewRuntime(&stubBroker{sub: subscriber}, nil)
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	router, err := runtime.NewRouter()
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if err := runtime.Start(router, mqx.SubscriptionSpec{
		Destination: "test.user",
		Group:       "restart",
		OnExhausted: mqx.ExhaustedDiscard,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for subscriber.calls() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := subscriber.calls(); got < 2 {
		t.Fatalf("subscribe calls = %d, want >= 2（订阅异常终止后应重建）", got)
	}
}

type stubBroker struct {
	sub mqx.Subscriber
}

func (b *stubBroker) Publisher() (mqx.Publisher, error)   { return nil, mqx.ErrNotSupported }
func (b *stubBroker) Subscriber() (mqx.Subscriber, error) { return b.sub, nil }
func (b *stubBroker) Ping(context.Context) error          { return nil }
func (b *stubBroker) Close(context.Context) error         { return nil }

// failingSubscriber 每次订阅都立即以致命错误终止。
type failingSubscriber struct {
	mu    sync.Mutex
	count int
}

func (s *failingSubscriber) Subscribe(context.Context, mqx.SubscriptionSpec, mqx.Handler) (mqx.Subscription, error) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()

	errs := make(chan error, 1)
	errs <- errors.New("fatal")
	close(errs)
	return &stubSubscription{errs: errs, closed: true}, nil
}

func (s *failingSubscriber) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

type stubSubscription struct {
	mu     sync.Mutex
	closed bool
	errs   chan error
}

func (s *stubSubscription) Err() <-chan error { return s.errs }

func (s *stubSubscription) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.errs)
	}
	return nil
}

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("等待消息超时")
		var zero T
		return zero
	}
}

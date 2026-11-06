package rabbitmqx

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// topology 声明 Exchange/Queue/Binding。声明是幂等的，重连后可重复执行。
type topology struct {
	exchangeType string
	durable      bool
	autoDelete   bool
}

func newTopology(cfg *Config) *topology {
	return &topology{
		exchangeType: cfg.ExchangeType,
		durable:      !cfg.Transient,
		autoDelete:   cfg.Transient,
	}
}

func (t *topology) declareExchange(ch *amqp.Channel, exchange string) error {
	if exchange == "" {
		return fmt.Errorf("exchange is required")
	}
	return ch.ExchangeDeclare(
		exchange,
		t.exchangeType,
		t.durable,
		t.autoDelete,
		false, // internal
		false, // noWait
		nil,
	)
}

// declareQueue 声明队列并按 binding 绑定到 exchange，返回队列名。
func (t *topology) declareQueue(ch *amqp.Channel, exchange, queue, binding string) (string, error) {
	if err := t.declareExchange(ch, exchange); err != nil {
		return "", fmt.Errorf("declare exchange: %w", err)
	}
	if binding == "" {
		binding = "#"
	}
	declared, err := ch.QueueDeclare(
		queue,
		t.durable,
		t.autoDelete,
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(declared.Name, binding, exchange, false, nil); err != nil {
		return "", fmt.Errorf("bind queue: %w", err)
	}
	return declared.Name, nil
}

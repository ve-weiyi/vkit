package rabbitmqx

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// topology 管理 RabbitMQ 的 Exchange/Queue/Binding 声明与重连恢复。
type topology struct {
	exchange   string
	exType     string
	durable    bool
	autoDelete bool
}

// newTopology 创建拓扑管理器。
func newTopology(exchange, exType string, durable, autoDelete bool) *topology {
	return &topology{
		exchange:   exchange,
		exType:     exType,
		durable:    durable,
		autoDelete: autoDelete,
	}
}

// Setup 声明 Exchange（幂等），返回 channel 供后续使用。
func (t *topology) Setup(ch *amqp.Channel) error {
	return ch.ExchangeDeclare(
		t.exchange,
		t.exType,
		t.durable,
		t.autoDelete,
		false, // internal
		false, // noWait
		nil,   // args
	)
}

// SetupQueue 声明 Exchange + Queue + Binding，返回队列名。
func (t *topology) SetupQueue(ch *amqp.Channel, queueName, routingKey string) (string, error) {
	if err := t.Setup(ch); err != nil {
		return "", fmt.Errorf("declare exchange: %w", err)
	}

	q, err := ch.QueueDeclare(
		queueName,
		t.durable,
		t.autoDelete,
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return "", fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.QueueBind(q.Name, routingKey, t.exchange, false, nil); err != nil {
		return "", fmt.Errorf("bind queue: %w", err)
	}

	return q.Name, nil
}

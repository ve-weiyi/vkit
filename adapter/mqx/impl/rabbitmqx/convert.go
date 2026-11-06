package rabbitmqx

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// toAMQP 将 mqx.Message 转为 amqp.Publishing。
func toAMQP(msg *mqx.Message) amqp.Publishing {
	headers := make(amqp.Table, len(msg.Headers))
	for k, v := range msg.Headers {
		headers[k] = v
	}
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	return amqp.Publishing{
		MessageId:   msg.ID,
		ContentType: "application/octet-stream",
		Body:        msg.Body,
		Headers:     headers,
		Timestamp:   ts,
	}
}

// fromAMQP 将 amqp.Delivery 转为 mqx.Message。exchange 是订阅声明的目的地。
func fromAMQP(exchange string, d amqp.Delivery) mqx.Message {
	headers := make(map[string]string, len(d.Headers))
	for k, v := range d.Headers {
		if s, ok := v.(string); ok {
			headers[k] = s
		} else {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}
	return mqx.Message{
		ID:          d.MessageId,
		Destination: exchange,
		RoutingKey:  d.RoutingKey,
		Headers:     headers,
		Body:        d.Body,
		Timestamp:   d.Timestamp,
	}
}

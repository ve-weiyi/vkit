package rabbitmqx

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// toAMQP 将 mqx.Message 转为 amqp.Publishing。
// mergeHeaders 是 PublishOption 中追加的 headers，会与 Message.Headers 合并。
// 若 msg 为 nil，返回空的 amqp.Publishing。
func toAMQP(msg *mqx.Message, headers map[string]string) amqp.Publishing {
	if msg == nil {
		msg = &mqx.Message{}
	}
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	// 合并 headers
	h := amqp.Table{}
	for k, v := range msg.Headers {
		h[k] = v
	}
	for k, v := range headers {
		h[k] = v
	}

	return amqp.Publishing{
		MessageId:   msg.ID,
		ContentType: "application/octet-stream",
		Body:        msg.Body,
		Headers:     h,
		Timestamp:   ts,
	}
}

// fromAMQP 将 amqp.Delivery 转为 mqx.Message。
func fromAMQP(d amqp.Delivery) *mqx.Message {
	headers := make(map[string]string, len(d.Headers))
	for k, v := range d.Headers {
		if s, ok := v.(string); ok {
			headers[k] = s
		} else {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}

	return &mqx.Message{
		ID:        d.MessageId,
		Key:       d.RoutingKey,
		Body:      d.Body,
		Headers:   headers,
		Timestamp: d.Timestamp,
	}
}

package kafkax

import (
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// toHeaders 把业务头与信封字段写入 Kafka record headers。
func toHeaders(msg *mqx.Message) []kgo.RecordHeader {
	out := make([]kgo.RecordHeader, 0, len(msg.Headers)+2)
	for k, v := range msg.Headers {
		if k == mqx.HeaderEventType || k == mqx.HeaderEventID {
			continue
		}
		out = append(out, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	if msg.RoutingKey != "" {
		out = append(out, kgo.RecordHeader{Key: mqx.HeaderEventType, Value: []byte(msg.RoutingKey)})
	}
	if msg.ID != "" {
		out = append(out, kgo.RecordHeader{Key: mqx.HeaderEventID, Value: []byte(msg.ID)})
	}
	return out
}

// fromRecord 把 Kafka record 转为 mqx.Message。
// ID 缺失时用 topic-partition-offset 兜底，保证消费侧始终有稳定的幂等键。
func fromRecord(record *kgo.Record) mqx.Message {
	headers := make(map[string]string, len(record.Headers))
	var routingKey, id string
	for _, h := range record.Headers {
		value := string(h.Value)
		headers[h.Key] = value
		switch h.Key {
		case mqx.HeaderEventType:
			routingKey = value
		case mqx.HeaderEventID:
			id = value
		}
	}
	if id == "" {
		id = fmt.Sprintf("%s-%d-%d", record.Topic, record.Partition, record.Offset)
	}
	timestamp := record.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return mqx.Message{
		ID:          id,
		Destination: record.Topic,
		RoutingKey:  routingKey,
		Key:         string(record.Key),
		Headers:     headers,
		Body:        record.Value,
		Timestamp:   timestamp,
	}
}

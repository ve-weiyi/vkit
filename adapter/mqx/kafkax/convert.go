package kafkax

import (
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// toKafkaRecord 将 mqx.Message 转换为 kgo.Record。
// 若 msg 为 nil，返回空的 kgo.Record。
func toKafkaRecord(msg *mqx.Message, mergeHeaders map[string]string) *kgo.Record {
	if msg == nil {
		return &kgo.Record{Timestamp: time.Now()}
	}
	r := &kgo.Record{
		Key:       []byte(msg.Key),
		Value:     msg.Body,
		Timestamp: msg.Timestamp,
	}
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}

	// Headers
	for k, v := range msg.Headers {
		r.Headers = append(r.Headers, kgo.RecordHeader{
			Key:   k,
			Value: []byte(v),
		})
	}
	for k, v := range mergeHeaders {
		r.Headers = append(r.Headers, kgo.RecordHeader{
			Key:   k,
			Value: []byte(v),
		})
	}

	return r
}

// fromKafkaRecord 将 kgo.Record 转换为 mqx.Message。
func fromKafkaRecord(r *kgo.Record) *mqx.Message {
	if r == nil {
		return &mqx.Message{}
	}
	msg := &mqx.Message{
		Key:       string(r.Key),
		Body:      r.Value,
		Timestamp: r.Timestamp,
		Headers:   make(map[string]string, len(r.Headers)),
	}
	for _, h := range r.Headers {
		msg.Headers[h.Key] = string(h.Value)
	}
	return msg
}

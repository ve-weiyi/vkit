package redisstreamx

import (
	"fmt"
	"strings"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

const (
	fieldID        = "id"
	fieldRouting   = "rk"
	fieldKey       = "key"
	fieldBody      = "body"
	fieldTimestamp = "ts"
	headerPrefix   = "h_"
)

func toRedis(msg *mqx.Message) map[string]any {
	fields := map[string]any{
		fieldRouting:   msg.RoutingKey,
		fieldKey:       msg.Key,
		fieldBody:      string(msg.Body),
		fieldTimestamp: msg.Timestamp.Format(time.RFC3339Nano),
	}
	if msg.ID != "" {
		fields[fieldID] = msg.ID
	}
	for k, v := range msg.Headers {
		fields[headerPrefix+k] = v
	}
	return fields
}

// fromRedis 把 stream 消息转为 mqx.Message。业务 ID 缺失时退回 stream ID。
func fromRedis(stream, streamID string, values map[string]any) mqx.Message {
	msg := mqx.Message{
		ID:          streamID,
		Destination: stream,
		Headers:     make(map[string]string, len(values)),
	}
	for k, v := range values {
		s := toString(v)
		switch {
		case k == fieldID && s != "":
			msg.ID = s
		case k == fieldRouting:
			msg.RoutingKey = s
		case k == fieldKey:
			msg.Key = s
		case k == fieldBody:
			msg.Body = []byte(s)
		case k == fieldTimestamp:
			if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
				msg.Timestamp = ts
			}
		case strings.HasPrefix(k, headerPrefix):
			msg.Headers[strings.TrimPrefix(k, headerPrefix)] = s
		}
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	return msg
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

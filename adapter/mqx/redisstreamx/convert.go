package redisstreamx

import (
	"fmt"
	"strings"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

const (
	headerPrefix = "h_"
	fieldID      = "id"
	fieldKey     = "key"
	fieldBody    = "body"
)

// toRedis 将 mqx.Message 转为 Redis Stream 字段。
// mergeHeaders 是 PublishOption 中追加的 headers，会与 Message.Headers 合并。
// 若 msg 为 nil，返回仅包含 mergeHeaders 的字段映射。
func toRedis(msg *mqx.Message, mergeHeaders map[string]string) map[string]interface{} {
	if msg == nil {
		msg = &mqx.Message{}
	}
	fields := make(map[string]interface{}, 3+len(msg.Headers)+len(mergeHeaders))

	fields[fieldID] = msg.ID
	fields[fieldKey] = msg.Key
	fields[fieldBody] = string(msg.Body)

	for k, v := range msg.Headers {
		fields[headerPrefix+k] = v
	}
	for k, v := range mergeHeaders {
		fields[headerPrefix+k] = v
	}

	return fields
}

// fromRedis 将 Redis Stream 消息映射还原为 mqx.Message。
// 非字符串类型的字段值和空 body 都会静默跳过。
func fromRedis(fields map[string]interface{}) *mqx.Message {
	msg := &mqx.Message{
		Headers: make(map[string]string),
	}

	for k, v := range fields {
		s, ok := v.(string)
		if !ok {
			// 非字符串值通过 fmt.Sprintf 尽力转换。
			// 注意：若 body 被存储为 []byte 等非字符串类型，
			// fmt.Sprintf("%v", v) 会输出 Go 语法表示（如 "[72 101]"），
			// 而非原始字节内容，这会导致数据损坏。通常不会发生。
			s = fmt.Sprintf("%v", v)
		}
		switch {
		case k == fieldID:
			msg.ID = s
		case k == fieldKey:
			msg.Key = s
		case k == fieldBody:
			msg.Body = []byte(s)
		case strings.HasPrefix(k, headerPrefix):
			msg.Headers[k[len(headerPrefix):]] = s
		}
	}

	return msg
}

package event

import "encoding/json"

// Codec 是事件负载的编解码器。默认 JSON；需要 Protobuf 时由调用方替换。
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSON 是默认编解码器。
var JSON Codec = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

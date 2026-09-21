package httpx

import "encoding/json"

// Unmarshal 将响应体反序列化为 v。
func (r *Response) Unmarshal(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// CURL 生成等价的 curl 命令，便于调试。
func (r *Response) CURL() string {
	if r.Request != nil {
		return r.Request.CURL()
	}
	return ""
}

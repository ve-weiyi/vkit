package httpx

import "fmt"

// Error 表示非 2xx 响应的 HTTP 错误。
// Body 字段已截断（最多 defaultMaxBodyLogLen），如需完整响应体请使用 WithStream() 手动读取。
type Error struct {
	StatusCode int
	Body       string
}

func (e *Error) Error() string {
	return fmt.Sprintf("httpx: %d %s", e.StatusCode, e.Body)
}

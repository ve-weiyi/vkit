package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	ansiReset = "\033[0m"
	ansiGreen = "\033[32m"
	ansiRed   = "\033[31m"
)

var sensitiveHeaders = []string{"authorization", "api-key", "x-api-key", "apikey", "token"}

// newLoggingTransport 包装 http.RoundTripper，记录详细的请求和响应。
// w 为输出目标，nil 时默认 os.Stderr。
func newLoggingTransport(transport http.RoundTripper, w io.Writer) *loggingTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if w == nil {
		w = os.Stderr
	}
	return &loggingTransport{
		transport:  transport,
		w:          w,
		maxBodyLen: defaultMaxBodyLogLen,
	}
}

type loggingTransport struct {
	transport  http.RoundTripper
	w          io.Writer
	maxBodyLen int
}

// RoundTrip 执行请求并记录日志。会读取并替换 req.Body（标准 RoundTripper 行为），
// 内容保持不变，可安全重放。
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.logRequest(req)
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		t.printf("\n%s\n\n", t.red(fmt.Sprintf("[HTTP ERROR] %v", err)))
		return resp, err
	}
	t.logResponse(resp)
	return resp, err
}

func (t *loggingTransport) logRequest(req *http.Request) {
	t.printf("\n%s\n", t.green("========== [HTTP REQUEST] =========="))
	t.printf("%s\n", t.green(fmt.Sprintf("Method: %s", req.Method)))
	t.printf("%s\n", t.green(fmt.Sprintf("URL: %s", req.URL.String())))
	t.printf("%s\n", t.green("Headers:"))
	for key, values := range req.Header {
		if t.isSensitive(key) {
			t.printf("%s\n", t.green(fmt.Sprintf("  %s: [REDACTED]", key)))
		} else {
			t.printf("%s\n", t.green(fmt.Sprintf("  %s: %s", key, strings.Join(values, ", "))))
		}
	}
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		} else {
			// 读取失败时仍替换 body，拼接已读部分避免数据丢失
			req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), req.Body))
		}
		body := truncateBody(string(bodyBytes), t.maxBodyLen)
		t.printf("%s\n%s\n", t.green("Body:"), t.green(body))
	}
	t.printf("%s\n", t.green("==========================================="))
}

func (t *loggingTransport) logResponse(resp *http.Response) {
	t.printf("\n%s\n", t.red("========== [HTTP RESPONSE] =========="))
	t.printf("%s\n", t.red(fmt.Sprintf("Status: %s", resp.Status)))

	contentType := resp.Header.Get("Content-Type")
	isStreaming := strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "stream")
	if isStreaming {
		t.printf("%s\n\n", t.red("Body: [STREAMING]"))
		resp.Body = &loggingReadCloser{ReadCloser: resp.Body, t: t, isFirst: true}
		return
	}
	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		} else {
			resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), resp.Body))
		}
		body := truncateBody(string(bodyBytes), t.maxBodyLen)
		t.printf("%s\n%s\n", t.red("Body:"), t.red(body))
	}
	t.printf("%s\n\n", t.red("=============================================="))
}

func (t *loggingTransport) isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveHeaders {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func (t *loggingTransport) printf(format string, v ...any) {
	fmt.Fprintf(t.w, format, v...)
}

func (t *loggingTransport) green(s string) string { return ansiGreen + s + ansiReset }
func (t *loggingTransport) red(s string) string   { return ansiRed + s + ansiReset }

// --- streaming body wrapper ---

type loggingReadCloser struct {
	io.ReadCloser
	t       *loggingTransport
	isFirst bool
}

func (l *loggingReadCloser) Read(p []byte) (n int, err error) {
	n, err = l.ReadCloser.Read(p)
	if n > 0 {
		if l.isFirst {
			l.t.printf("%s\n", l.t.red("[STREAMING CHUNKS]"))
			l.isFirst = false
		}
		data := p[:n]
		for len(data) > 0 {
			i := bytes.IndexByte(data, '\n')
			var line []byte
			if i >= 0 {
				line = data[:i]
				data = data[i+1:]
			} else {
				line = data
				data = nil
			}
			if bytes.HasPrefix(line, []byte("data: ")) {
				l.t.printf("%s\n", l.t.red(string(line)))
			}
		}
	}
	return n, err
}

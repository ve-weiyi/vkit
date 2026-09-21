package httpx

import (
	"net/http"
	"testing"
	"time"
)

func TestWithQuery(t *testing.T) {
	var c config
	WithQuery("page", "1")(&c)
	WithQuery("size", "10")(&c)

	if c.req.query["page"] != "1" {
		t.Errorf("expected page=1, got %q", c.req.query["page"])
	}
	if c.req.query["size"] != "10" {
		t.Errorf("expected size=10, got %q", c.req.query["size"])
	}
	if len(c.req.query) != 2 {
		t.Errorf("expected 2 query params, got %d", len(c.req.query))
	}
}

func TestWithQueryNilMap(t *testing.T) {
	var c config
	if c.req.query != nil {
		t.Error("expected nil query map before WithQuery")
	}
	WithQuery("key", "val")(&c)
	if c.req.query == nil {
		t.Error("expected non-nil query map after WithQuery")
	}
}

func TestWithHeader(t *testing.T) {
	var c config
	WithHeader("X-Custom", "hello")(&c)
	WithHeader("Authorization", "Bearer token")(&c)

	if c.req.headers["X-Custom"] != "hello" {
		t.Errorf("expected X-Custom=hello, got %q", c.req.headers["X-Custom"])
	}
	if c.req.headers["Authorization"] != "Bearer token" {
		t.Errorf("expected Authorization=Bearer token, got %q", c.req.headers["Authorization"])
	}
	if len(c.req.headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(c.req.headers))
	}
}

func TestWithHeaderNilMap(t *testing.T) {
	var c config
	if c.req.headers != nil {
		t.Error("expected nil headers map before WithHeader")
	}
	WithHeader("Key", "Val")(&c)
	if c.req.headers == nil {
		t.Error("expected non-nil headers map after WithHeader")
	}
}

func TestWithJSON(t *testing.T) {
	var c config
	type payload struct {
		Name  string
		Value int
	}
	p := payload{Name: "test", Value: 42}
	WithJSON(p)(&c)

	if c.req.json == nil {
		t.Fatal("expected non-nil json")
	}
	got, ok := c.req.json.(payload)
	if !ok {
		t.Fatalf("expected payload type, got %T", c.req.json)
	}
	if got.Name != "test" || got.Value != 42 {
		t.Errorf("unexpected json value: %+v", got)
	}
}

func TestWithJSONNil(t *testing.T) {
	var c config
	WithJSON(nil)(&c)
	if c.req.json != nil {
		t.Errorf("expected nil json, got %v", c.req.json)
	}
}

func TestWithForm(t *testing.T) {
	var c config
	f := map[string]string{"key": "value", "foo": "bar"}
	WithForm(f)(&c)

	if c.req.form == nil {
		t.Fatal("expected non-nil form")
	}
	if c.req.form["key"] != "value" {
		t.Errorf("expected key=value, got %q", c.req.form["key"])
	}
	if c.req.form["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %q", c.req.form["foo"])
	}
}

func TestWithBody(t *testing.T) {
	var c config
	data := []byte("raw body content")
	WithBody(data)(&c)

	if c.req.bodyData == nil {
		t.Fatal("expected non-nil bodyData")
	}
	if string(c.req.bodyData) != "raw body content" {
		t.Errorf("expected 'raw body content', got %q", string(c.req.bodyData))
	}
}

func TestWithBodyEmpty(t *testing.T) {
	var c config
	WithBody([]byte{})(&c)

	if c.req.bodyData == nil {
		t.Fatal("expected non-nil bodyData (empty slice)")
	}
	if len(c.req.bodyData) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(c.req.bodyData))
	}
}

func TestWithStream(t *testing.T) {
	var c config
	if c.req.stream {
		t.Error("expected stream=false by default")
	}
	WithStream()(&c)
	if !c.req.stream {
		t.Error("expected stream=true after WithStream")
	}
}

func TestOptionComposition(t *testing.T) {
	var c config
	type payload struct{ Msg string }

	WithQuery("q", "search")(&c)
	WithHeader("X-Trace", "abc123")(&c)
	WithJSON(payload{Msg: "hello"})(&c)
	WithStream()(&c)

	if c.req.query["q"] != "search" {
		t.Errorf("query not set: %v", c.req.query)
	}
	if c.req.headers["X-Trace"] != "abc123" {
		t.Errorf("header not set: %v", c.req.headers)
	}
	if c.req.json == nil {
		t.Error("json not set")
	}
	if !c.req.stream {
		t.Error("stream not set")
	}
}

// TestBuildTransportInsecureSkipWithRoundTripper 验证 WithInsecureSkipVerify
// 在 WithRoundTripper 的 Transport 上生效，而非被覆盖。
func TestBuildTransportInsecureSkipWithRoundTripper(t *testing.T) {
	customTransport := &http.Transport{}
	cfg := &config{}
	cfg.client.transport = DefaultTransport()
	cfg.client.timeout = 30 * time.Second
	WithRoundTripper(customTransport)(cfg)
	WithInsecureSkipVerify()(cfg)

	rt := buildTransport(cfg)

	tr, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", rt)
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be set")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
	if tr != customTransport {
		t.Error("expected the custom transport to be preserved")
	}
}

// TestBuildTransportInsecureSkipWithoutRoundTripper 验证单独使用
// WithInsecureSkipVerify 时，TLS 配置应用到 DefaultTransport。
func TestBuildTransportInsecureSkipWithoutRoundTripper(t *testing.T) {
	cfg := &config{}
	cfg.client.transport = DefaultTransport()
	cfg.client.timeout = 30 * time.Second
	WithInsecureSkipVerify()(cfg)

	rt := buildTransport(cfg)

	tr, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", rt)
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be set")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

// TestBuildTransportLoggingTransport 验证 LoggingTransport 包裹最终 Transport。
func TestBuildTransportLoggingTransport(t *testing.T) {
	var buf mockWriter
	cfg := &config{}
	cfg.client.transport = DefaultTransport()
	cfg.client.timeout = 30 * time.Second
	WithLoggingTransport(&buf)(cfg)

	rt := buildTransport(cfg)

	lt, ok := rt.(*loggingTransport)
	if !ok {
		t.Fatalf("expected *loggingTransport, got %T", rt)
	}
	if lt.w != &buf {
		t.Error("expected logging output writer to be set")
	}
	if lt.transport == nil {
		t.Error("expected inner transport to be set")
	}
}

type mockWriter struct{}

func (m *mockWriter) Write(p []byte) (n int, err error) { return len(p), nil }

package httpx

import (
	"net/http"
	"testing"
	"time"
)

func TestRequestOptions(t *testing.T) {
	t.Run("query additive", func(t *testing.T) {
		var c config
		WithQuery("page", "1")(&c)
		WithQuery("size", "10")(&c)

		if got := c.req.query["page"]; got != "1" {
			t.Errorf("page = %q, want 1", got)
		}
		if got := c.req.query["size"]; got != "10" {
			t.Errorf("size = %q, want 10", got)
		}
		if len(c.req.query) != 2 {
			t.Errorf("query 数量 = %d, want 2", len(c.req.query))
		}
	})

	t.Run("header additive", func(t *testing.T) {
		var c config
		WithHeader("X-Custom", "hello")(&c)
		WithHeader("Authorization", "Bearer token")(&c)

		if got := c.req.headers["X-Custom"]; got != "hello" {
			t.Errorf("X-Custom = %q, want hello", got)
		}
		if got := c.req.headers["Authorization"]; got != "Bearer token" {
			t.Errorf("Authorization = %q, want Bearer token", got)
		}
		if len(c.req.headers) != 2 {
			t.Errorf("header 数量 = %d, want 2", len(c.req.headers))
		}
	})

	t.Run("json", func(t *testing.T) {
		var c config
		type payload struct{ Name string }
		WithJSON(payload{Name: "test"})(&c)

		got, ok := c.req.json.(payload)
		if !ok {
			t.Fatalf("json 类型 = %T, want payload", c.req.json)
		}
		if got.Name != "test" {
			t.Errorf("json = %+v, want Name=test", got)
		}
	})

	t.Run("form", func(t *testing.T) {
		var c config
		WithForm(map[string]string{"key": "value"})(&c)

		if got := c.req.form["key"]; got != "value" {
			t.Errorf("form[key] = %q, want value", got)
		}
	})

	t.Run("body", func(t *testing.T) {
		var c config
		WithBody([]byte("raw body"))(&c)

		if got := string(c.req.bodyData); got != "raw body" {
			t.Errorf("body = %q, want %q", got, "raw body")
		}
	})

	t.Run("stream", func(t *testing.T) {
		var c config
		WithStream()(&c)

		if !c.req.stream {
			t.Error("stream 未开启")
		}
	})
}

func TestBuildTransport(t *testing.T) {
	t.Run("insecure skip with custom round tripper", func(t *testing.T) {
		custom := &http.Transport{}
		cfg := &config{}
		cfg.client.transport = DefaultTransport()
		cfg.client.timeout = 30 * time.Second
		WithRoundTripper(custom)(cfg)
		WithInsecureSkipVerify()(cfg)

		tr, ok := buildTransport(cfg).(*http.Transport)
		if !ok {
			t.Fatalf("transport 类型 = %T, want *http.Transport", buildTransport(cfg))
		}
		if tr != custom {
			t.Error("自定义 transport 被覆盖")
		}
		if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify 未生效")
		}
	})

	t.Run("insecure skip uses default transport", func(t *testing.T) {
		cfg := &config{}
		cfg.client.transport = DefaultTransport()
		WithInsecureSkipVerify()(cfg)

		tr, ok := buildTransport(cfg).(*http.Transport)
		if !ok {
			t.Fatalf("transport 类型 = %T, want *http.Transport", buildTransport(cfg))
		}
		if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify 未生效")
		}
	})

	t.Run("logging transport wraps inner transport", func(t *testing.T) {
		var buf mockWriter
		cfg := &config{}
		cfg.client.transport = DefaultTransport()
		WithLoggingTransport(&buf)(cfg)

		lt, ok := buildTransport(cfg).(*loggingTransport)
		if !ok {
			t.Fatalf("transport 类型 = %T, want *loggingTransport", buildTransport(cfg))
		}
		if lt.w != &buf || lt.transport == nil {
			t.Error("logging transport 未正确装配")
		}
	})
}

type mockWriter struct{}

func (m *mockWriter) Write(p []byte) (n int, err error) { return len(p), nil }

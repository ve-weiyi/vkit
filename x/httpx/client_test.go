package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ve-weiyi/vkit/x/httpx"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	resp, err := client.Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("unexpected body: %s", string(resp.Body))
	}

	var result map[string]interface{}
	if err := resp.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result["ok"] != true {
		t.Errorf("unexpected value: %v", result["ok"])
	}
}

func TestClientPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	resp, err := client.Post(t.Context(), "/test", httpx.WithJSON(map[string]string{"a": "1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func TestClientStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("streaming data"))
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	resp, err := client.Get(t.Context(), "/test", httpx.WithStream())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Response.Body.Close()
	data, err := io.ReadAll(resp.Response.Body)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "streaming data" {
		t.Errorf("unexpected data: %s", string(data))
	}
	if len(resp.Body) != 0 {
		t.Errorf("stream mode: expected empty resp.Body ([]byte), got %d bytes", len(resp.Body))
	}
}

func TestClientWithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value1" {
			t.Errorf("unexpected X-Custom: %s", r.Header.Get("X-Custom"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("unexpected Authorization: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(
		httpx.WithBaseURL(srv.URL),
		httpx.WithHeaders(map[string]string{
			"X-Custom":      "value1",
			"Authorization": "Bearer token",
		}),
	)
	_, err := client.Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithQueryIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "page=1&size=10" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	_, err := client.Get(t.Context(), "/test", httpx.WithQuery("page", "1"), httpx.WithQuery("size", "10"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithHeaderPerRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "hello" {
			t.Errorf("unexpected header: %s", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	_, err := client.Get(t.Context(), "/test", httpx.WithHeader("X-Custom", "hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithJSONRoundTrip(t *testing.T) {
	type payload struct{ Name string }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected Content-Type: %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"name":"test"}`))
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	resp, err := client.Post(t.Context(), "/test", httpx.WithJSON(payload{Name: "test"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result payload
	if err := resp.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("unexpected name: %s", result.Name)
	}
}

func TestWithFormIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form error: %v", err)
		}
		if r.FormValue("key") != "value" {
			t.Errorf("unexpected form value: %s", r.FormValue("key"))
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	_, err := client.Post(t.Context(), "/test", httpx.WithForm(map[string]string{"key": "value"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithBodyIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	_, err := client.Post(t.Context(), "/test", httpx.WithBody([]byte("raw body")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := httpx.New(httpx.WithBaseURL(srv.URL))
	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpxErr, ok := err.(*httpx.Error)
	if !ok {
		t.Fatalf("expected *httpx.Error, got %T", err)
	}
	if httpxErr.StatusCode != 500 {
		t.Errorf("unexpected status: %d", httpxErr.StatusCode)
	}
	if httpxErr.Body != "internal error" {
		t.Errorf("unexpected body: %s", httpxErr.Body)
	}
}

// TestWithHeadersRequestScoped 请求级 WithHeaders 只作用于该次请求。
// 旧实现里请求级 Option 会直接写 client 共享的 headers map：既污染该 client 的
// 后续请求，并发时还是数据竞争。
func TestWithHeadersRequestScoped(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("X-Test"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpx.New(httpx.WithBaseURL(srv.URL))

	if _, err := c.Get(context.Background(), "/a", httpx.WithHeaders(map[string]string{"X-Test": "one"})); err != nil {
		t.Fatalf("第一次 Get: %v", err)
	}
	if _, err := c.Get(context.Background(), "/b"); err != nil {
		t.Fatalf("第二次 Get: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("期望 2 次请求，实际 %d", len(got))
	}
	if got[0] != "one" {
		t.Errorf("第一次请求 X-Test = %q, want %q", got[0], "one")
	}
	if got[1] != "" {
		t.Errorf("第二次请求不应带上一次请求的 X-Test，实际 %q", got[1])
	}
}

// TestWithHeadersClientDefault 客户端级 WithHeaders 应作用于每个请求。
func TestWithHeadersClientDefault(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("X-Default"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpx.New(
		httpx.WithBaseURL(srv.URL),
		httpx.WithHeaders(map[string]string{"X-Default": "d"}),
	)

	for _, path := range []string{"/a", "/b"} {
		if _, err := c.Get(context.Background(), path); err != nil {
			t.Fatalf("Get %s: %v", path, err)
		}
	}

	for i, v := range got {
		if v != "d" {
			t.Errorf("第 %d 次请求 X-Default = %q, want %q", i+1, v, "d")
		}
	}
}

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
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, err := httpx.New(httpx.WithBaseURL(srv.URL)).Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("body = %s", resp.Body)
	}

	var result map[string]any
	if err := resp.Unmarshal(&result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
}

func TestClientStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("streaming data"))
	}))
	defer srv.Close()

	resp, err := httpx.New(httpx.WithBaseURL(srv.URL)).Get(t.Context(), "/test", httpx.WithStream())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Response.Body.Close()

	data, err := io.ReadAll(resp.Response.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "streaming data" {
		t.Errorf("data = %q, want %q", data, "streaming data")
	}
	if len(resp.Body) != 0 {
		t.Errorf("stream 模式下 Body 应为空，实际 %d 字节", len(resp.Body))
	}
}

func TestClientErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := httpx.New(httpx.WithBaseURL(srv.URL)).Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("500 响应应返回错误")
	}
	httpxErr, ok := err.(*httpx.Error)
	if !ok {
		t.Fatalf("error 类型 = %T, want *httpx.Error", err)
	}
	if httpxErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", httpxErr.StatusCode)
	}
	if httpxErr.Body != "internal error" {
		t.Errorf("body = %q, want %q", httpxErr.Body, "internal error")
	}
}

// TestRequestOptionsIntegration 校验请求级 Option 最终落到实际 HTTP 请求上。
func TestRequestOptionsIntegration(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.RawQuery; got != "page=1&size=10" {
				t.Errorf("query = %q, want page=1&size=10", got)
			}
		}))
		defer srv.Close()

		_, err := httpx.New(httpx.WithBaseURL(srv.URL)).Get(t.Context(), "/test",
			httpx.WithQuery("page", "1"), httpx.WithQuery("size", "10"))
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
	})

	t.Run("header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-Custom"); got != "hello" {
				t.Errorf("X-Custom = %q, want hello", got)
			}
		}))
		defer srv.Close()

		_, err := httpx.New(httpx.WithBaseURL(srv.URL)).Get(t.Context(), "/test",
			httpx.WithHeader("X-Custom", "hello"))
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
	})

	t.Run("json", func(t *testing.T) {
		type payload struct{ Name string }

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"test"}`))
		}))
		defer srv.Close()

		resp, err := httpx.New(httpx.WithBaseURL(srv.URL)).Post(t.Context(), "/test",
			httpx.WithJSON(payload{Name: "test"}))
		if err != nil {
			t.Fatalf("Post: %v", err)
		}
		var result payload
		if err := resp.Unmarshal(&result); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if result.Name != "test" {
			t.Errorf("name = %q, want test", result.Name)
		}
	})

	t.Run("form", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %q", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("key"); got != "value" {
				t.Errorf("key = %q, want value", got)
			}
		}))
		defer srv.Close()

		_, err := httpx.New(httpx.WithBaseURL(srv.URL)).Post(t.Context(), "/test",
			httpx.WithForm(map[string]string{"key": "value"}))
		if err != nil {
			t.Fatalf("Post: %v", err)
		}
	})

	t.Run("body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "raw body" {
				t.Errorf("body = %q, want raw body", body)
			}
		}))
		defer srv.Close()

		_, err := httpx.New(httpx.WithBaseURL(srv.URL)).Post(t.Context(), "/test",
			httpx.WithBody([]byte("raw body")))
		if err != nil {
			t.Fatalf("Post: %v", err)
		}
	})
}

// TestClientHeadersScope 请求级 headers 不污染 client，客户端级 headers 作用于每个请求。
func TestClientHeadersScope(t *testing.T) {
	t.Run("request scoped", func(t *testing.T) {
		var got []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = append(got, r.Header.Get("X-Test"))
		}))
		defer srv.Close()

		c := httpx.New(httpx.WithBaseURL(srv.URL))
		if _, err := c.Get(context.Background(), "/a", httpx.WithHeaders(map[string]string{"X-Test": "one"})); err != nil {
			t.Fatalf("第一次 Get: %v", err)
		}
		if _, err := c.Get(context.Background(), "/b"); err != nil {
			t.Fatalf("第二次 Get: %v", err)
		}

		if len(got) != 2 || got[0] != "one" || got[1] != "" {
			t.Errorf("X-Test 序列 = %v, want [one ]", got)
		}
	})

	t.Run("client default", func(t *testing.T) {
		var got []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = append(got, r.Header.Get("X-Default"))
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

		if len(got) != 2 || got[0] != "d" || got[1] != "d" {
			t.Errorf("X-Default 序列 = %v, want [d d]", got)
		}
	})
}

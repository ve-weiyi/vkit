package httpx_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpx2 "github.com/ve-weiyi/vkit/x/httpx"
)

// TestRetryOn5xx 验证 5xx 响应会触发重试，最终成功。
func TestRetryOn5xx(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 3, Delay: 10 * time.Millisecond}),
	)

	resp, err := client.Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("unexpected body: %s", string(resp.Body))
	}
	if count != 3 {
		t.Errorf("expected 3 server hits, got %d", count)
	}
	if resp.Attempt != 3 {
		t.Errorf("expected attempt 3, got %d", resp.Attempt)
	}
}

// TestRetryOnNetworkError 验证网络错误会触发重试。
func TestRetryOnNetworkError(t *testing.T) {
	// 使用一个已关闭的端口模拟网络错误
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	var attempts []int
	client := httpx2.New(
		httpx2.WithBaseURL("http://"+addr),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 3, Delay: 10 * time.Millisecond}),
		httpx2.WithMiddleware(func(next httpx2.Handler) httpx2.Handler {
			return func(ctx context.Context, req *httpx2.Request) (*httpx2.Response, error) {
				attempts = append(attempts, req.Attempt)
				return next(ctx, req)
			}
		}),
	)

	_, err = client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d: %v", len(attempts), attempts)
	}
	if attempts[0] != 1 || attempts[1] != 2 || attempts[2] != 3 {
		t.Errorf("unexpected attempt numbers: %v", attempts)
	}
}

// TestNoRetryOn4xx 验证 4xx 响应不触发重试。
func TestNoRetryOn4xx(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 3, Delay: 10 * time.Millisecond}),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpxErr, ok := err.(*httpx2.Error)
	if !ok {
		t.Fatalf("expected *httpx.Error, got %T", err)
	}
	if httpxErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", httpxErr.StatusCode)
	}
	if count != 1 {
		t.Errorf("expected 1 server hit (no retry on 4xx), got %d", count)
	}
}

// TestNoRetryOnStream 验证流式模式不触发重试。
func TestNoRetryOnStream(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 3, Delay: 10 * time.Millisecond}),
	)

	_, err := client.Get(t.Context(), "/test", httpx2.WithStream())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpxErr, ok := err.(*httpx2.Error)
	if !ok {
		t.Fatalf("expected *httpx.Error, got %T", err)
	}
	if httpxErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpxErr.StatusCode)
	}
	if count != 1 {
		t.Errorf("expected 1 server hit (no retry on stream), got %d", count)
	}
}

// TestRetryExhausted 验证重试耗尽后返回最后一次错误。
func TestRetryExhausted(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 2, Delay: 10 * time.Millisecond}),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpxErr, ok := err.(*httpx2.Error)
	if !ok {
		t.Fatalf("expected *httpx.Error, got %T", err)
	}
	if httpxErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpxErr.StatusCode)
	}
	if count != 2 {
		t.Errorf("expected 2 server hits (max attempts), got %d", count)
	}
}

// TestRetryMaxAttemptsOne 验证 MaxAttempts=1 不触发重试。
func TestRetryMaxAttemptsOne(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithRetry(httpx2.RetryConfig{MaxAttempts: 1, Delay: 10 * time.Millisecond}),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if count != 1 {
		t.Errorf("expected 1 server hit (no retry), got %d", count)
	}
}

// TestRetryMiddlewareNoRetry 验证直接使用 RetryMiddleware + MaxAttempts=0 不重试。
func TestRetryMiddlewareNoRetry(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithMiddleware(httpx2.RetryMiddleware(httpx2.RetryConfig{MaxAttempts: 0})),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if count != 1 {
		t.Errorf("expected 1 server hit (no retry), got %d", count)
	}
}

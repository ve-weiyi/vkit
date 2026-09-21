package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpx2 "github.com/ve-weiyi/vkit/x/httpx"
)

func TestHooks(t *testing.T) {
	var onReqCalled, onRespCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithHooks(&httpx2.Hooks{
			OnRequest: func(req *httpx2.Request) {
				onReqCalled = true
			},
			OnResponse: func(resp *httpx2.Response) {
				onRespCalled = true
			},
		}),
	)

	_, err := client.Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !onReqCalled {
		t.Error("OnRequest not called")
	}
	if !onRespCalled {
		t.Error("OnResponse not called")
	}
}

func TestHooksOnError(t *testing.T) {
	var onErrCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithHooks(&httpx2.Hooks{
			OnError: func(req *httpx2.Request, err error) {
				onErrCalled = true
			},
		}),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !onErrCalled {
		t.Error("OnError not called")
	}
}

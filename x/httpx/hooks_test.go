package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ve-weiyi/vkit/x/httpx"
)

func TestHooks(t *testing.T) {
	var onReqCalled, onRespCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx.New(
		httpx.WithBaseURL(srv.URL),
		httpx.WithHooks(&httpx.Hooks{
			OnRequest: func(req *httpx.Request) {
				onReqCalled = true
			},
			OnResponse: func(resp *httpx.Response) {
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

	client := httpx.New(
		httpx.WithBaseURL(srv.URL),
		httpx.WithHooks(&httpx.Hooks{
			OnError: func(req *httpx.Request, err error) {
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

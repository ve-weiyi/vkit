package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx2 "github.com/ve-weiyi/vkit/x/httpx"
)

func TestCURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx2.New(httpx2.WithBaseURL(srv.URL))
	resp, err := client.Get(t.Context(), "/test", httpx2.WithQuery("a", "1"), httpx2.WithHeader("X-Test", "hi"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	curl := resp.Request.CURL()
	if !strings.Contains(curl, "curl") {
		t.Errorf("expected curl command, got: %s", curl)
	}
	if !strings.Contains(curl, "GET") {
		t.Errorf("expected GET method: %s", curl)
	}
	if !strings.Contains(curl, "X-Test: hi") {
		t.Errorf("expected X-Test header: %s", curl)
	}
	if !strings.Contains(curl, "/test?a=1") {
		t.Errorf("expected request URL with query: %s", curl)
	}
}

func TestCURLWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := httpx2.New(httpx2.WithBaseURL(srv.URL))
	resp, err := client.Post(t.Context(), "/test", httpx2.WithJSON(map[string]string{"key": "value"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	curl := resp.Request.CURL()
	if !strings.Contains(curl, "curl") {
		t.Errorf("expected curl command, got: %s", curl)
	}
	if !strings.Contains(curl, "POST") {
		t.Errorf("expected POST method: %s", curl)
	}
	if !strings.Contains(curl, `"key":"value"`) {
		t.Errorf("expected body in curl: %s", curl)
	}
	if !strings.Contains(curl, "--data-raw") {
		t.Errorf("expected --data-raw flag: %s", curl)
	}
}

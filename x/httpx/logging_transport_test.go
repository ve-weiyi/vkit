package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx2 "github.com/ve-weiyi/vkit/x/httpx"
)

func TestLoggingTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// verify body was readable after logging
		body, _ := io.ReadAll(r.Body)
		var v map[string]string
		json.Unmarshal(body, &v)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithLoggingTransport(&buf),
	)

	resp, err := client.Post(t.Context(), "/test", httpx2.WithJSON(map[string]string{"a": "1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}

	output := buf.String()
	// request section
	if !strings.Contains(output, "[HTTP REQUEST]") {
		t.Error("missing request header")
	}
	if !strings.Contains(output, "POST") {
		t.Error("missing method")
	}
	if !strings.Contains(output, "[REDACTED]") {
		// Authorization header shouldn't be present for this test
	}
	// response section
	if !strings.Contains(output, "[HTTP RESPONSE]") {
		t.Error("missing response header")
	}
	if !strings.Contains(output, "200 OK") {
		t.Error("missing status")
	}
}

func TestLoggingTransportStreaming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte("data: hello\n"))
		w.Write([]byte("data: world\n"))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithLoggingTransport(&buf),
	)

	resp, err := client.Get(t.Context(), "/test", httpx2.WithStream())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Response.Body.Close()

	io.ReadAll(resp.Response.Body)

	output := buf.String()
	if !strings.Contains(output, "[STREAMING]") {
		t.Error("missing streaming marker")
	}
	if !strings.Contains(output, "[STREAMING CHUNKS]") {
		t.Error("missing streaming chunks header")
	}
}

func TestLoggingTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithLoggingTransport(&buf),
	)

	_, err := client.Get(t.Context(), "/test")
	if err == nil {
		t.Fatal("expected error")
	}

	output := buf.String()
	if !strings.Contains(output, "[HTTP REQUEST]") {
		t.Error("missing request section")
	}
	if !strings.Contains(output, "[HTTP RESPONSE]") {
		t.Error("missing response section for error status")
	}
}

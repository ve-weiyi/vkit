package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx2 "github.com/ve-weiyi/vkit/x/httpx"
)

type testLogger struct {
	logs []string
}

func (l *testLogger) Printf(format string, v ...any) {
	l.logs = append(l.logs, fmt.Sprintf(format, v...))
}

func TestLoggerMiddleware(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	logger := &testLogger{}
	client := httpx2.New(
		httpx2.WithBaseURL(srv.URL),
		httpx2.WithMiddleware(httpx2.LoggerMiddleware(logger)),
	)

	_, err := client.Get(t.Context(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, l := range logger.logs {
		if strings.Contains(l, "GET /test") && strings.Contains(l, "200") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected log entry not found: %v", logger.logs)
	}
}

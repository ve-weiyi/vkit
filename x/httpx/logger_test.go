package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ve-weiyi/vkit/x/httpx"
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
	client := httpx.New(
		httpx.WithBaseURL(srv.URL),
		httpx.WithMiddleware(httpx.LoggerMiddleware(logger)),
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

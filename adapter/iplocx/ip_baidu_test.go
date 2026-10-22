package iplocx

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetIpInfoByBaiduCaches(t *testing.T) {
	var calls atomic.Int32
	original := httpClient
	defer func() { httpClient = original }()

	httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := `{"status":"0","data":[{"location":"广东省深圳市"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	cacheMu.Lock()
	clear(ipCache)
	cacheMu.Unlock()

	first, err := GetIpInfoByBaidu("1.2.3.4")
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	if first.Location != "广东省深圳市" {
		t.Fatalf("location = %q", first.Location)
	}

	second, err := GetIpInfoByBaidu("1.2.3.4")
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if second.Location != first.Location {
		t.Fatalf("cached location = %q, want %q", second.Location, first.Location)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("http calls = %d, want 1（第二次应命中缓存）", got)
	}
}

package music

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ve-weiyi/vkit/plugins/music/netease"
)

type fakeServer struct {
	searchCalls int
	songErr     error
}

func (f *fakeServer) Search(_ context.Context, keyword string) ([]*netease.Song, error) {
	f.searchCalls++
	return []*netease.Song{{Id: "1", Name: keyword}}, nil
}

func (f *fakeServer) Song(_ context.Context, id string) (*netease.Song, error) {
	if f.songErr != nil {
		return nil, f.songErr
	}
	return &netease.Song{Id: id, Name: "song"}, nil
}

func (f *fakeServer) SongLink(_ context.Context, id string) (*netease.SongLink, error) {
	return &netease.SongLink{Id: id}, nil
}

func (f *fakeServer) Lyric(_ context.Context, id string) (*netease.Lyric, error) {
	return &netease.Lyric{Lyric: "[00:00]lyric"}, nil
}

func (f *fakeServer) Album(_ context.Context, id string) (*netease.Album, error) {
	return &netease.Album{Id: id}, nil
}

func (f *fakeServer) Artist(_ context.Context, id string) (*netease.Artist, error) {
	return &netease.Artist{Id: id}, nil
}

func (f *fakeServer) Playlist(_ context.Context, id string) (*netease.Playlist, error) {
	return &netease.Playlist{Id: id}, nil
}

func request(t *testing.T, handler http.HandlerFunc, target string) (*httptest.ResponseRecorder, interface{}) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, target, nil))

	var body interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %q", rec.Body.String())
	}
	return rec, body
}

// 同一关键词在 TTL 内只打一次上游；大小写不同也命中同一缓存。
func TestHandler_SearchCachesCaseInsensitively(t *testing.T) {
	fake := &fakeServer{}
	handler := newMusicPlugin(fake).Handler("/api/v1/music")

	first, _ := request(t, handler, "/api/v1/music/search?keyword=Jay")
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}
	if got := first.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=300") {
		t.Fatalf("Cache-Control = %q, want max-age=300", got)
	}

	second, _ := request(t, handler, "/api/v1/music/search?keyword=jay")
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", second.Code)
	}
	if fake.searchCalls != 1 {
		t.Fatalf("upstream 调用 %d 次，期望 1 次", fake.searchCalls)
	}
}

// ID 必须为纯数字，防止拼进上游 URL 路径。
func TestHandler_RejectsInvalidID(t *testing.T) {
	handler := newMusicPlugin(&fakeServer{}).Handler("/api/v1/music")

	for _, target := range []string{
		"/api/v1/music/song",
		"/api/v1/music/song?id=abc",
		"/api/v1/music/album?id=1/2",
	} {
		rec, body := request(t, handler, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
		if body.(map[string]interface{})["error"] != "invalid_param" {
			t.Errorf("%s: error = %v, want invalid_param", target, body.(map[string]interface{})["error"])
		}
	}
}

func TestHandler_RejectsInvalidKeyword(t *testing.T) {
	handler := newMusicPlugin(&fakeServer{}).Handler("/api/v1/music")

	for _, target := range []string{
		"/api/v1/music/search",
		"/api/v1/music/search?keyword=",
		"/api/v1/music/search?keyword=" + strings.Repeat("a", 51),
	} {
		rec, _ := request(t, handler, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

// 上游失败返回 JSON 错误，而不是纯文本 502。
func TestHandler_UpstreamErrorIsJSON(t *testing.T) {
	handler := newMusicPlugin(&fakeServer{songErr: errors.New("boom")}).Handler("/api/v1/music")

	rec, body := request(t, handler, "/api/v1/music/song?id=1")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if body.(map[string]interface{})["error"] != "upstream_unavailable" {
		t.Fatalf("error = %v, want upstream_unavailable", body.(map[string]interface{})["error"])
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("错误响应不应被缓存: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestHandler_UnknownPath(t *testing.T) {
	handler := newMusicPlugin(&fakeServer{}).Handler("/api/v1/music")

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/music/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

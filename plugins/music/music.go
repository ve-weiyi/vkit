package music

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/singleflight"

	"github.com/ve-weiyi/vkit/plugins/music/netease"
)

const (
	// searchTTL 搜索结果变化快，短缓存即可挡住输入防抖带来的重复请求。
	searchTTL = 5 * time.Minute
	// metaTTL 歌曲、专辑、歌手、歌单是稳定元数据。
	metaTTL = time.Hour
	// lyricTTL 歌词基本不变。
	lyricTTL = 24 * time.Hour
	// linkTTL 播放地址会过期，缓存时间必须短于其可用窗口。
	linkTTL = 5 * time.Minute

	maxKeyword = 50
	cacheSize  = 1024
)

var idPattern = regexp.MustCompile(`^[0-9]+$`)

type MusicPlugin struct {
	server netease.Server
	cache  *ttlCache
	group  singleflight.Group
}

func NewMusicPlugin() *MusicPlugin {
	return newMusicPlugin(netease.New())
}

func newMusicPlugin(server netease.Server) *MusicPlugin {
	return &MusicPlugin{server: server, cache: newTTLCache(cacheSize)}
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (p *MusicPlugin) Handler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")
		ctx := r.Context()

		switch path {
		case "search":
			keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
			if n := utf8.RuneCountInString(keyword); n == 0 || n > maxKeyword {
				writeError(w, http.StatusBadRequest, "invalid_param", "keyword 长度必须在 1-50 之间")
				return
			}
			p.serve(w, "search:"+strings.ToLower(keyword), searchTTL, func() (interface{}, error) {
				return p.server.Search(ctx, keyword)
			})
		case "song":
			serveID(p, w, r, "song", metaTTL, p.server.Song)
		case "song/link":
			serveID(p, w, r, "song:link", linkTTL, p.server.SongLink)
		case "lyric":
			serveID(p, w, r, "lyric", lyricTTL, p.server.Lyric)
		case "album":
			serveID(p, w, r, "album", metaTTL, p.server.Album)
		case "artist":
			serveID(p, w, r, "artist", metaTTL, p.server.Artist)
		case "playlist":
			serveID(p, w, r, "playlist", metaTTL, p.server.Playlist)
		default:
			http.NotFound(w, r)
		}
	}
}

func serveID[T any](p *MusicPlugin, w http.ResponseWriter, r *http.Request, name string, ttl time.Duration, load func(context.Context, string) (T, error)) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid_param", "id 必须是数字")
		return
	}
	ctx := r.Context()
	p.serve(w, name+":"+id, ttl, func() (interface{}, error) {
		return load(ctx, id)
	})
}

// serve 先查缓存，未命中时用 singleflight 合并并发请求，再把结果写入响应。
func (p *MusicPlugin) serve(w http.ResponseWriter, key string, ttl time.Duration, load func() (interface{}, error)) {
	if body, ok := p.cache.get(key); ok {
		writeJSON(w, body, ttl)
		return
	}

	value, err, _ := p.group.Do(key, func() (interface{}, error) {
		data, err := load()
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		p.cache.set(key, body, ttl)
		return body, nil
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "音乐服务暂不可用")
		return
	}

	body, _ := value.([]byte)
	writeJSON(w, body, ttl)
}

func writeJSON(w http.ResponseWriter, body []byte, ttl time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(ttl.Seconds())))
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: code, Message: message})
}

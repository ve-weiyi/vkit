package music

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ve-weiyi/vkit/plugins/music/netease"
)

type MusicPlugin struct {
	server netease.Server
}

func NewMusicPlugin() *MusicPlugin {
	return &MusicPlugin{
		server: netease.New(),
	}
}

func (p *MusicPlugin) Handler(prefix string) http.HandlerFunc {
	server := p.server
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		keyword := r.URL.Query().Get("keyword")

		var data interface{}
		var err error

		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")
		switch path {
		case "search":
			data, err = server.Search(keyword)
		case "song":
			data, err = server.Song(id)
		case "song/link":
			data, err = server.SongLink(id)
		case "lyric":
			data, err = server.Lyric(id)
		case "album":
			data, err = server.Album(id)
		case "artist":
			data, err = server.Artist(id)
		case "playlist":
			data, err = server.Playlist(id)
		default:
			http.NotFound(w, r)
			return
		}

		// 上游调用失败必须在这里就返回：否则会被下面的 json.Marshal 覆盖，
		// 把失败当成 200 + null 交给调用方
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		body, err := json.Marshal(data)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}

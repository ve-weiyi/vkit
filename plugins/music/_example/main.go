package main

import (
	"log"
	"net/http"

	"github.com/ve-weiyi/vkit/plugins/music"
)

func main() {

	http.HandleFunc("/api/v1", music.NewMusicPlugin().Handler("/music/").ServeHTTP)

	log.Println("Music server starting on :8080")
	log.Println("Endpoints:")
	log.Println("  GET /api/v1/music/search?keyword=xxx")
	log.Println("  GET /api/v1/music/song?id=xxx")
	log.Println("  GET /api/v1/music/song/link?id=xxx")
	log.Println("  GET /api/v1/music/lyric?id=xxx")
	log.Println("  GET /api/v1/music/album?id=xxx")
	log.Println("  GET /api/v1/music/artist?id=xxx")
	log.Println("  GET /api/v1/music/playlist?id=xxx")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

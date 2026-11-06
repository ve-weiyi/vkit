package netease

import (
	"context"
	"strings"
)

type (
	Server interface {
		Search(context.Context, string) ([]*Song, error)
		Song(context.Context, string) (*Song, error)
		SongLink(context.Context, string) (*SongLink, error)
		Lyric(context.Context, string) (*Lyric, error)
		Album(context.Context, string) (*Album, error)
		Artist(context.Context, string) (*Artist, error)
		Playlist(context.Context, string) (*Playlist, error)
	}
)

type (
	Playlist struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Songs Songs  `json:"songs"`
	}
	Artist struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Songs Songs  `json:"songs"`
	}
	Album struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Songs Songs  `json:"songs"`
	}
	Lyric struct {
		Lyric string `json:"lyric"`
		Trans string `json:"trans"`
	}
	Song struct {
		Id      string    `json:"id"`
		Name    string    `json:"name"`
		Picture string    `json:"picture"`
		Artists []*Artist `json:"artists"`
		Album   *Album    `json:"album"`
	}
	Songs    []*Song
	SongLink struct {
		Id   string `json:"id"`
		Br   int64  `json:"br"`
		Size int64  `json:"size"`
		URL  string `json:"url"`
	}
)

func (s *Song) GetArtist() string {
	authors := make([]string, len(s.Artists))
	for i, art := range s.Artists {
		authors[i] = art.Name
	}
	return strings.Join(authors, "/")
}

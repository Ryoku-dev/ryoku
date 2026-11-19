// Package plan is the hand-off between the resolvers and the engine: what one
// download fetches and what happens to the parts once they have landed.
package plan

import (
	"context"
	"net/http"
)

type Kind string

const (
	KindFile    Kind = "file"
	KindMedia   Kind = "media"
	KindTorrent Kind = "torrent"
	KindPage    Kind = "page"
)

type PartKind string

const (
	PartHTTP  PartKind = "http"
	PartHLS   PartKind = "hls"
	PartYTDLP PartKind = "ytdlp"
)

type Part struct {
	Kind    PartKind
	URL     string
	Ext     string
	Size    int64
	Headers http.Header
	// "video", "audio", "file", "item", or "" for one complete stream
	Role string
	// PartYTDLP only: format selector and extra arguments for yt-dlp
	Args []string
	// Role "item" only: file name inside the folder Plan.Name
	Name string
}

type Post string

const (
	PostNone      Post = ""
	PostMerge     Post = "merge"
	PostRemux     Post = "remux"
	PostMute      Post = "mute"
	PostAudio     Post = "audio"
	PostTranscode Post = "transcode"
)

type Plan struct {
	Name      string
	Container string
	Parts     []Part
	Post      Post
}

type Progress interface {
	SetTotal(bytes int64)
	Add(bytes int64)
	// unit index of count has reached fill (0..1); drives the piece map
	Mark(index, count int, fill float64)
	Stage(name string)
}

type Limiter interface {
	WaitN(ctx context.Context, n int) error
}

type Env struct {
	HTTP   *http.Client
	Limit  Limiter
	Cache  string
	YTDLP  string
	FFmpeg string
}

package engine

import (
	"context"

	"golang.org/x/time/rate"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

// The engine runs work through these so it can be tested without the network
// and wired to the real resolvers, fetchers and torrent client in one place.

type Resolver interface {
	Resolve(ctx context.Context, url string) (*media.Media, error)
	Plan(m *media.Media, opts media.Options) (*plan.Plan, error)
}

type Fetcher interface {
	Fetch(ctx context.Context, part plan.Part, dst string, p plan.Progress) error
}

type Finisher interface {
	Finish(ctx context.Context, pl *plan.Plan, inputs []string, out string) error
}

type Torrents interface {
	// ctx bounds only the add step; the torrent lives until Pause, Drop or Close
	Add(ctx context.Context, src, dir string, selected []int) (Torrent, error)
	Resolve(ctx context.Context, src string) (name string, files []proto.File, err error)
	SetSeed(on bool, ratio float64, minutes int)
	// clears a dropped torrent's kept state (its src) through the open client
	Forget(src string) error
	Close() error
}

type Torrent interface {
	Metadata() <-chan struct{}
	Name() string
	Path() string
	Files() []proto.File
	Select(selected []int) error
	Pause()
	Resume()
	Drop(deleteData bool) error
	Stats() TorrentStats
	// replaced when a selection adds files, so read them again after Select
	Completed() <-chan struct{}
	Finished() <-chan struct{}
	Err() error
}

type TorrentStats struct {
	Size, Done, Down, Up, Uploaded int64
	Peers, Seeds, Conns            int
	Ratio                          float64
	Pieces                         [proto.PieceBuckets]byte
	Complete, Seeding              bool
}

// NewTorrents builds the torrent client; down and up are the limiters HTTP
// shares, so one cap covers everything.
type NewTorrents func(s proto.Settings, down, up *rate.Limiter) (Torrents, error)

// Parts is what wire.go hands the engine before Start.
type Parts struct {
	Resolver Resolver
	Fetchers map[plan.PartKind]Fetcher
	Finisher Finisher
	Torrents NewTorrents
	// caches a remote thumbnail and returns its local path, or ""
	Thumb func(ctx context.Context, url string) string
	// clears the torrent client's kept state for a dropped torrent (its src);
	// called only while no client is open
	Forget func(src string) error
}

package main

import (
	"context"
	"path/filepath"

	"golang.org/x/time/rate"

	"ryoget/internal/bt"
	"ryoget/internal/engine"
	"ryoget/internal/hls"
	"ryoget/internal/media"
	_ "ryoget/internal/media/services"
	"ryoget/internal/mux"
	"ryoget/internal/paths"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
	"ryoget/internal/thumbs"
	"ryoget/internal/ytdlp"
)

// wire is the one place the engine meets the concrete resolvers, fetchers,
// muxer and torrent client.
func wire(e *engine.Engine) engine.Parts {
	env := e.Env()
	return engine.Parts{
		Resolver: resolver{e: e},
		Fetchers: map[plan.PartKind]engine.Fetcher{
			plan.PartHLS: fetchFunc(func(ctx context.Context, part plan.Part, dst string, p plan.Progress) error {
				return hls.Fetch(ctx, env, part, dst, p)
			}),
			plan.PartYTDLP: fetchFunc(func(ctx context.Context, part plan.Part, dst string, p plan.Progress) error {
				return ytdlp.Fetch(ctx, env, part, dst, p)
			}),
		},
		Finisher: finisher{env: env},
		Torrents: func(s proto.Settings, down, up *rate.Limiter) (engine.Torrents, error) {
			c, err := bt.New(bt.Config{
				StateDir: torrentState(),
				CacheDir: paths.CacheDir(),
				Port:     s.Port,
				DHT:      s.DHT,
				PEX:      true,
				Down:     down,
				Up:       up,
				Seed:     bt.SeedPolicy{On: s.Seed, Ratio: s.SeedRatio, Minutes: s.SeedMinutes},
				HTTP:     env.HTTP,
			})
			if err != nil {
				return nil, err
			}
			return btClient{c}, nil
		},
		Forget: func(src string) error { return bt.Forget(torrentState(), src) },
		Thumb: func(ctx context.Context, url string) string {
			p, err := thumbs.Fetch(ctx, env.HTTP, thumbs.DefaultDir(), url)
			if err != nil {
				return ""
			}
			return p
		},
	}
}

func torrentState() string { return filepath.Join(paths.StateDir(), "torrents") }

type resolver struct{ e *engine.Engine }

func (r resolver) Resolve(ctx context.Context, url string) (*media.Media, error) {
	return media.Resolve(ctx, url, r.e.Env(), r.e.Settings().YtdlpFallback)
}

func (resolver) Plan(m *media.Media, opts media.Options) (*plan.Plan, error) {
	return media.PlanFor(m, opts)
}

type fetchFunc func(ctx context.Context, part plan.Part, dst string, p plan.Progress) error

func (f fetchFunc) Fetch(ctx context.Context, part plan.Part, dst string, p plan.Progress) error {
	return f(ctx, part, dst, p)
}

type finisher struct{ env *plan.Env }

func (f finisher) Finish(ctx context.Context, pl *plan.Plan, inputs []string, out string) error {
	return mux.Finish(ctx, f.env.FFmpeg, pl, inputs, out)
}

type btClient struct{ c *bt.Client }

func (b btClient) Add(ctx context.Context, src, dir string, selected []int) (engine.Torrent, error) {
	t, err := b.c.Add(ctx, src, dir, selected)
	if err != nil {
		return nil, err
	}
	return btTorrent{t}, nil
}

func (b btClient) Resolve(ctx context.Context, src string) (string, []proto.File, error) {
	return b.c.Resolve(ctx, src)
}

func (b btClient) SetSeed(on bool, ratio float64, minutes int) {
	b.c.SetSeed(bt.SeedPolicy{On: on, Ratio: ratio, Minutes: minutes})
}

func (b btClient) Forget(src string) error { return b.c.Forget(src) }

func (b btClient) Close() error { return b.c.Close() }

type btTorrent struct{ *bt.Torrent }

func (t btTorrent) Stats() engine.TorrentStats {
	s := t.Torrent.Stats()
	return engine.TorrentStats{
		Size: s.Size, Done: s.Done, Down: s.Down, Up: s.Up, Uploaded: s.Uploaded,
		Peers: s.Peers, Seeds: s.Seeds, Conns: s.Conns, Ratio: s.Ratio, Pieces: s.Pieces,
		Complete: s.Complete, Seeding: s.Seeding,
	}
}

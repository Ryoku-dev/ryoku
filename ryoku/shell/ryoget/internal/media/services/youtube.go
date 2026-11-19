package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/youtube"
	"ryoget/internal/ytdlp"

	i18n "ryoku-i18n"
)

func init() { media.Register(ytService{}) }

// ytResolveFallback is swapped by tests; it is yt-dlp in production.
var ytResolveFallback = ytdlp.Resolve

type ytService struct{}

func (ytService) ID() string   { return "youtube" }
func (ytService) Name() string { return "YouTube" }

func (ytService) Match(u *url.URL) bool { return youtube.IsYouTube(u.Hostname()) }

// Resolve reads the video natively; when YouTube refuses the native client
// (bot gate, made-for-kids, age gate, no plain URLs, an API error) and the
// caller allows it, yt-dlp gets the link. The result is settled either way so
// the generic chain does not run yt-dlp a second time. A playlist link plans
// its first video (or the one the link names) and lists every entry as Items.
func (ytService) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	id, list := youtube.ParseURL(u)
	music := strings.HasPrefix(strings.ToLower(u.Hostname()), "music.")
	if id == "" && list == "" {
		return nil, media.Settled(fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("This YouTube link is not a video or a playlist.")))
	}
	base := "https://www.youtube.com/watch?v="
	if music {
		base = "https://music.youtube.com/watch?v="
	}
	yt := youtube.For(ytHTTP(env), ytCache(env))
	var pl *youtube.Playlist
	// a mix (RD...) is an endless radio, not a list worth offering
	if list != "" && (id == "" || !strings.HasPrefix(list, "RD")) {
		var err error
		if pl, err = yt.Playlist(ctx, list); err != nil && id == "" {
			return ytFallback(ctx, u, env, err)
		}
		if id == "" {
			id = pl.First
		}
	}
	m, err := yt.Media(ctx, id)
	if err != nil {
		watch, _ := url.Parse(youtube.WatchURL(id))
		if m, err = ytFallback(ctx, watch, env, err); err != nil {
			return nil, err
		}
	}
	m.URL = base + id
	if music {
		m.Source.Name = "YouTube Music"
	}
	if pl != nil {
		for _, e := range pl.Entries {
			m.Items = append(m.Items, media.Item{Kind: "video", URL: base + e.ID, Title: e.Title, Thumb: e.Thumb})
		}
		if pl.Count > len(pl.Entries) {
			m.Note = i18n.Tf("%d videos, the first %d listed", pl.Count, len(pl.Entries))
		} else {
			m.Note = i18n.Tf("%d videos", pl.Count)
		}
	}
	return m, nil
}

func ytFallback(ctx context.Context, u *url.URL, env *plan.Env, native error) (*media.Media, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !media.YTDLPAllowed(ctx) || errors.Is(native, youtube.ErrGone) || errors.Is(native, media.ErrPrivate) ||
		errors.Is(native, media.ErrUnsupported) {
		return nil, media.Settled(native)
	}
	m, err := ytResolveFallback(ctx, env, u.String())
	if err != nil {
		// yt-dlp missing says nothing about the video; keep YouTube's reason
		if errors.Is(err, media.ErrUnsupported) {
			return nil, media.Settled(native)
		}
		return nil, media.Settled(err)
	}
	m.Source = media.Source{ID: "youtube", Name: "YouTube"}
	return m, nil
}

func ytHTTP(env *plan.Env) *http.Client {
	if env == nil {
		return nil
	}
	return env.HTTP
}

func ytCache(env *plan.Env) string {
	if env == nil {
		return ""
	}
	return env.Cache
}

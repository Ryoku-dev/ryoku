package services

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var streamableAPI = "https://api.streamable.com"

type streamable struct{}

func init() { media.Register(streamable{}) }

func (streamable) ID() string   { return "streamable" }
func (streamable) Name() string { return "Streamable" }

func (streamable) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "streamable.com") }

type streamableFile struct {
	URL       string  `json:"url"`
	Height    int     `json:"height"`
	Bitrate   int     `json:"bitrate"`
	Size      int64   `json:"size"`
	Duration  float64 `json:"duration"`
	Framerate float64 `json:"framerate"`
}

func (s streamable) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	p := pathParts(u)
	if len(p) > 1 && (p[0] == "e" || p[0] == "o" || p[0] == "s") {
		p = p[1:]
	}
	if len(p) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := p[0]
	var v struct {
		Status       int                       `json:"status"`
		Title        string                    `json:"title"`
		ThumbnailURL string                    `json:"thumbnail_url"`
		Files        map[string]streamableFile `json:"files"`
	}
	if err := getJSON(ctx, env, streamableAPI+"/videos/"+url.PathEscape(id), nil, &v); err != nil {
		return nil, err
	}
	thumb := v.ThumbnailURL
	if strings.HasPrefix(thumb, "//") {
		thumb = "https:" + thumb
	}
	out := &media.Media{Kind: plan.KindMedia, URL: "https://streamable.com/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(v.Title, i18n.Tf("Streamable video %s", id)), Thumb: thumb}
	for _, key := range []string{"mp4", "mp4-mobile"} {
		f, ok := v.Files[key]
		if !ok || f.URL == "" {
			continue
		}
		fu := f.URL
		if strings.HasPrefix(fu, "//") {
			fu = "https:" + fu
		}
		out.Formats = append(out.Formats, fileVideo(key, fu, "mp4", f.Height, int(f.Framerate+0.5), f.Bitrate, f.Size, "h264,aac", nil))
		if out.Duration == 0 {
			out.Duration = f.Duration
		}
	}
	if len(out.Formats) == 0 {
		// status 1 is still processing; anything else has no playable file
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video is not ready to download."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

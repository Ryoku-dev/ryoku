package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var rutubeBase = "https://rutube.ru"

type rutube struct{}

func init() { media.Register(rutube{}) }

func (rutube) ID() string   { return "rutube" }
func (rutube) Name() string { return "Rutube" }

func (rutube) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "rutube.ru") }

var rutubeIDRe = regexp.MustCompile(`/(?:video(?:/private)?|play/embed|embed|shorts)/([0-9a-f]{32})`)

func (s rutube) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := rutubeIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := m[1]
	q := url.Values{"no_404": {"true"}, "pver": {"v2"}}
	if key := u.Query().Get("p"); key != "" {
		q.Set("p", key)
	}
	var o struct {
		Title        string  `json:"title"`
		Duration     float64 `json:"duration"`
		ThumbnailURL string  `json:"thumbnail_url"`
		Author       *struct {
			Name string `json:"name"`
		} `json:"author"`
		Balancer *struct {
			M3U8 string `json:"m3u8"`
		} `json:"video_balancer"`
		Detail *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"detail"`
	}
	r, err := fetch(ctx, env, request{url: rutubeBase + "/api/play/options/" + id + "/?" + q.Encode(),
		header: hdr("Accept", "application/json", "Referer", rutubeBase+"/"), accept: []int{403, 404}})
	if err != nil {
		return nil, err
	}
	if err := decode(r.body, &o); err != nil {
		return nil, err
	}
	if o.Balancer == nil || o.Balancer.M3U8 == "" {
		kind := media.ErrUnavailable
		if r.status == 403 {
			kind = media.ErrPrivate
		}
		msg := i18n.T("This video was removed or is private.")
		if o.Detail != nil {
			msg = first(o.Detail.Description, o.Detail.Name, msg)
		}
		return nil, fmt.Errorf("%w: %s", kind, msg)
	}
	vs, err := hls.Variants(ctx, env, o.Balancer.M3U8, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: rutubeBase + "/video/" + id + "/", Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(o.Title, id), Thumb: o.ThumbnailURL, Duration: o.Duration / 1000, Formats: dmDedupe(media.HLSFormats(vs, nil))}
	if o.Author != nil {
		out.Author = o.Author.Name
	}
	out.Default = best(out.Formats)
	return out, nil
}

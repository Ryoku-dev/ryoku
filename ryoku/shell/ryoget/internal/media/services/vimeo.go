package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var vimeoPlayer = "https://player.vimeo.com"

type vimeo struct{}

func init() { media.Register(vimeo{}) }

func (vimeo) ID() string   { return "vimeo" }
func (vimeo) Name() string { return "Vimeo" }

func (vimeo) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "vimeo.com") }

// the numeric id, optionally followed by the unlisted hash
var vimeoIDRe = regexp.MustCompile(`/(\d{5,})(?:/([0-9a-f]{6,}))?(?:/|$)`)

func (s vimeo) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := vimeoIDRe.FindStringSubmatch(u.Path + "/")
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id, hash := m[1], first(m[2], u.Query().Get("h"))
	cfg := vimeoPlayer + "/video/" + id + "/config"
	if hash != "" {
		cfg += "?h=" + url.QueryEscape(hash)
	}
	var c struct {
		Video struct {
			Title    string            `json:"title"`
			Duration float64           `json:"duration"`
			Thumbs   map[string]string `json:"thumbs"`
			ThumbURL string            `json:"thumbnail_url"`
			Owner    struct {
				Name string `json:"name"`
			} `json:"owner"`
		} `json:"video"`
		Request struct {
			Files struct {
				Progressive []struct {
					URL     string `json:"url"`
					Height  int    `json:"height"`
					FPS     int    `json:"fps"`
					Quality string `json:"quality"`
				} `json:"progressive"`
				HLS struct {
					DefaultCDN string `json:"default_cdn"`
					CDNs       map[string]struct {
						URL string `json:"url"`
					} `json:"cdns"`
				} `json:"hls"`
			} `json:"files"`
		} `json:"request"`
	}
	r, err := fetch(ctx, env, request{url: cfg, header: hdr("Accept", "application/json", "Referer", "https://vimeo.com/"+id), accept: []int{403}})
	if err != nil {
		return nil, err
	}
	if r.status == 403 {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This video is private or needs a password."))
	}
	if err := decode(r.body, &c); err != nil {
		return nil, err
	}
	out := &media.Media{Kind: plan.KindMedia, URL: "https://vimeo.com/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(c.Video.Title, id), Author: c.Video.Owner.Name, Duration: c.Video.Duration, Thumb: first(c.Video.ThumbURL, vimeoThumb(c.Video.Thumbs))}
	for _, p := range c.Request.Files.Progressive {
		out.Formats = append(out.Formats, fileVideo("mp4-"+first(p.Quality, strconv.Itoa(p.Height)), p.URL, "mp4", p.Height, p.FPS, 0, 0, "h264,aac", nil))
	}
	h := c.Request.Files.HLS
	master := h.CDNs[h.DefaultCDN].URL
	if master == "" {
		for _, cdn := range h.CDNs {
			master = cdn.URL
			break
		}
	}
	if master != "" {
		vs, err := hls.Variants(ctx, env, master, nil)
		if err == nil {
			out.Formats = append(out.Formats, media.HLSFormats(vs, nil)...)
		} else if len(out.Formats) == 0 {
			return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream Vimeo will share."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func vimeoThumb(t map[string]string) string {
	if u := t["base"]; u != "" {
		return u
	}
	bestW, bestU := 0, ""
	for k, u := range t {
		if w, err := strconv.Atoi(k); err == nil && w > bestW {
			bestW, bestU = w, u
		}
	}
	return bestU
}

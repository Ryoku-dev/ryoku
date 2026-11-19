package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var dailymotionBase = "https://www.dailymotion.com"

type dailymotion struct{}

func init() { media.Register(dailymotion{}) }

func (dailymotion) ID() string   { return "dailymotion" }
func (dailymotion) Name() string { return "Dailymotion" }

func (dailymotion) Match(u *url.URL) bool {
	return media.HostIs(u.Hostname(), "dailymotion.com", "dai.ly")
}

var dmIDRe = regexp.MustCompile(`^/(?:embed/|swf/)?(?:video/)?(x[0-9a-z]{4,})`)

func (s dailymotion) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	id := u.Query().Get("video")
	if id == "" {
		if m := dmIDRe.FindStringSubmatch(u.Path); m != nil {
			id = m[1]
		}
	}
	if id == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	var md struct {
		Title    string            `json:"title"`
		Duration float64           `json:"duration"`
		Thumbs   map[string]string `json:"thumbnails"`
		Owner    struct {
			Screenname string `json:"screenname"`
		} `json:"owner"`
		Qualities map[string][]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"qualities"`
		Error *struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Title   string `json:"title"`
			Message string `json:"message"`
		} `json:"error"`
		Password bool `json:"is_password_protected"`
	}
	if err := getJSON(ctx, env, dailymotionBase+"/player/metadata/video/"+id, nil, &md); err != nil {
		return nil, err
	}
	if md.Error != nil {
		kind := media.ErrUnavailable
		if md.Password || strings.Contains(md.Error.Type, "private") || strings.Contains(md.Error.Type, "password") {
			kind = media.ErrPrivate
		}
		return nil, fmt.Errorf("%w: %s", kind, first(md.Error.Message, md.Error.Title, md.Error.Code))
	}
	master := ""
	for _, q := range md.Qualities["auto"] {
		if q.Type == "application/x-mpegURL" {
			master = q.URL
		}
	}
	if master == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream Dailymotion will share."))
	}
	// the CDN director refuses most requests that do not look like the web player's
	h := hdr("Referer", dailymotionBase+"/", "Origin", dailymotionBase, "Accept", "*/*", "Accept-Language", "en-US,en;q=0.5",
		"Sec-Fetch-Dest", "empty", "Sec-Fetch-Mode", "cors", "Sec-Fetch-Site", "same-site")
	vs, err := hls.Variants(ctx, env, master, h)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: dailymotionBase + "/video/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(md.Title, id), Author: md.Owner.Screenname, Duration: md.Duration, Thumb: dmPoster(md.Thumbs),
		Formats: dmDedupe(media.HLSFormats(vs, h))}
	out.Default = best(out.Formats)
	return out, nil
}

// the master lists every rendition once per CDN; one of each height is enough
func dmDedupe(fs []media.Format) []media.Format {
	seen := map[string]bool{}
	var out []media.Format
	for _, f := range fs {
		key := f.Kind + strconv.Itoa(f.Height) + strconv.Itoa(f.Bitrate)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func dmPoster(p map[string]string) string {
	bestH, bestU := 0, ""
	for k, u := range p {
		if h, err := strconv.Atoi(k); err == nil && h > bestH {
			bestH, bestU = h, u
		}
	}
	return bestU
}

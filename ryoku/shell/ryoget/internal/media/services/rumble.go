package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var rumbleBase = "https://rumble.com"

// rumble's embed JSON lists files per height, and its bot check lets only plain clients through.
type rumble struct{}

func init() { media.Register(rumble{}) }

func (rumble) ID() string   { return "rumble" }
func (rumble) Name() string { return "Rumble" }

func (rumble) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "rumble.com") }

var (
	rumbleEmbedRe = regexp.MustCompile(`rumble\.com\\?/embed\\?/(v[0-9a-z]+)`)
	rumblePageRe  = regexp.MustCompile(`^v[0-9a-z]+(?:-.*)?\.html$`)
)

type rumbleFile struct {
	URL  string `json:"url"`
	Meta struct {
		Bitrate int   `json:"bitrate"`
		Size    int64 `json:"size"`
		H       int   `json:"h"`
	} `json:"meta"`
}

func (s rumble) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	p := pathParts(u)
	id := ""
	switch {
	case len(p) >= 2 && p[0] == "embed":
		id = p[1]
	case len(p) == 1 && rumblePageRe.MatchString(p[0]):
		r, err := getBody(ctx, env, rumbleBase+"/"+p[0], hdr("User-Agent", plainUA, "Accept", "text/html"))
		if err != nil {
			return nil, err
		}
		if m := rumbleEmbedRe.FindSubmatch(r.body); m != nil {
			id = string(m[1])
		}
		if id == "" {
			return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This page has no player data."))
		}
	default:
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	var v struct {
		Title    string  `json:"title"`
		Thumb    string  `json:"i"`
		Link     string  `json:"l"`
		Duration float64 `json:"duration"`
		FPS      float64 `json:"fps"`
		Author   struct {
			Name string `json:"name"`
		} `json:"author"`
		Files map[string]map[string]rumbleFile `json:"ua"`
	}
	q := url.Values{"request": {"video"}, "ver": {"2"}, "v": {id}}
	if err := getJSON(ctx, env, rumbleBase+"/embedJS/u3/?"+q.Encode(), hdr("User-Agent", plainUA), &v); err != nil {
		return nil, err
	}
	page := rumbleBase + "/embed/" + id + "/"
	if strings.HasPrefix(v.Link, "/") {
		page = rumbleBase + v.Link
	}
	out := &media.Media{Kind: plan.KindMedia, URL: page, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(strings.TrimSpace(v.Title), i18n.Tf("Rumble video %s", id)), Author: v.Author.Name, Thumb: v.Thumb, Duration: v.Duration}
	fps := int(v.FPS + 0.5)
	for _, kind := range []string{"mp4", "webm"} {
		files := v.Files[kind]
		keys := make([]string, 0, len(files))
		for k := range files {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return atoiOr(keys[i]) > atoiOr(keys[j]) })
		for _, k := range keys {
			f := files[k]
			if f.URL == "" {
				continue
			}
			h := f.Meta.H
			if h == 0 {
				h = atoiOr(k)
			}
			codec := ""
			if kind == "mp4" {
				codec = "h264,aac"
			}
			out.Formats = append(out.Formats, fileVideo(kind+"-"+k, f.URL, kind, h, fps, f.Meta.Bitrate*1000, f.Meta.Size, codec, nil))
		}
	}
	for _, f := range v.Files["hls"] {
		if f.URL == "" {
			continue
		}
		vs, err := hls.Variants(ctx, env, f.URL, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		out.Formats = append(out.Formats, media.HLSFormats(vs, nil)...)
		break
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("No downloadable formats were found."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func atoiOr(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

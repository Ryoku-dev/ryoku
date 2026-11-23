package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

// ted talk pages name the HLS master and an mp4 in __NEXT_DATA__, YouTube-only talks go to that service.
type ted struct{}

func init() { media.Register(ted{}) }

func (ted) ID() string   { return "ted" }
func (ted) Name() string { return "TED" }

func (ted) Match(u *url.URL) bool {
	p := pathParts(u)
	return media.HostIs(u.Hostname(), "ted.com") && len(p) >= 2 && p[0] == "talks"
}

type tedVideo struct {
	Title        string  `json:"title"`
	Presenter    string  `json:"presenterDisplayName"`
	Duration     float64 `json:"duration"`
	CanonicalURL string  `json:"canonicalUrl"`
	HLSURL       string  `json:"hlsUrl"`
	Images       []struct {
		URL string `json:"url"`
	} `json:"primaryImageSet"`
	Player *struct {
		Thumb    string `json:"thumb"`
		External *struct {
			Service string `json:"service"`
			Code    string `json:"code"`
		} `json:"external"`
		Resources *struct {
			HLS *struct {
				Stream string `json:"stream"`
			} `json:"hls"`
			H264 []struct {
				Bitrate int    `json:"bitrate"`
				File    string `json:"file"`
			} `json:"h264"`
		} `json:"resources"`
	} `json:"videoPlayerData"`
}

func (s ted) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	slug := pathParts(u)[1]
	page := "https://www.ted.com/talks/" + url.PathEscape(slug)
	r, err := getBody(ctx, env, page, navigate())
	if err != nil {
		return nil, err
	}
	raw := scriptByID(r.body, "__NEXT_DATA__")
	if raw == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This page has no player data."))
	}
	var nd struct {
		Props struct {
			PageProps struct {
				VideoData *tedVideo `json:"videoData"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := decode(raw, &nd); err != nil {
		return nil, err
	}
	v := nd.Props.PageProps.VideoData
	if v == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single talk can be downloaded."))
	}
	thumb := ""
	if len(v.Images) > 0 {
		thumb = v.Images[0].URL
	}
	out := &media.Media{Kind: plan.KindMedia, URL: first(v.CanonicalURL, page), Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: v.Title, Author: v.Presenter, Duration: v.Duration, Thumb: thumb}
	stream := v.HLSURL
	if p := v.Player; p != nil {
		out.Thumb = first(out.Thumb, p.Thumb)
		if p.Resources != nil {
			if p.Resources.HLS != nil {
				stream = first(p.Resources.HLS.Stream, stream)
			}
			for _, f := range p.Resources.H264 {
				if f.File != "" {
					out.Formats = append(out.Formats, fileVideo("h264-"+strconv.Itoa(f.Bitrate)+"k", f.File, "mp4", 0, 0, f.Bitrate*1000, -1, "h264,aac", nil))
				}
			}
		}
	}
	if stream != "" {
		vs, err := hls.Variants(ctx, env, stream, nil)
		switch {
		case err == nil:
			out.Formats = append(media.HLSFormats(vs, nil), out.Formats...)
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case len(out.Formats) == 0 && errors.Is(err, media.ErrNetwork):
			return nil, err
		}
	}
	if len(out.Formats) == 0 {
		if p := v.Player; p != nil && p.External != nil && p.External.Service == "YouTube" && p.External.Code != "" {
			return youTubeTalk(ctx, env, p.External.Code)
		}
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("No downloadable formats were found."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func youTubeTalk(ctx context.Context, env *plan.Env, code string) (*media.Media, error) {
	yu, _ := url.Parse("https://www.youtube.com/watch?v=" + url.QueryEscape(code))
	svc := media.Lookup(yu)
	if svc == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("No downloadable formats were found."))
	}
	return svc.Resolve(ctx, yu, env)
}

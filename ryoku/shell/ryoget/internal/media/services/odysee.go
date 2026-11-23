package services

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var odyseeAPI = "https://api.na-backend.odysee.com/api/v1/proxy"

// odysee's public LBRY proxy resolves a claim and its get hands out the streaming URL.
type odysee struct{}

func init() { media.Register(odysee{}) }

func (odysee) ID() string   { return "odysee" }
func (odysee) Name() string { return "Odysee" }

func (odysee) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "odysee.com", "lbry.tv") }

type lbryClaim struct {
	CanonicalURL string `json:"canonical_url"`
	PermanentURL string `json:"permanent_url"`
	ValueType    string `json:"value_type"`
	Name         string `json:"name"`
	Channel      *struct {
		Name  string `json:"name"`
		Value struct {
			Title string `json:"title"`
		} `json:"value"`
	} `json:"signing_channel"`
	Value struct {
		Title      string `json:"title"`
		StreamType string `json:"stream_type"`
		Thumbnail  struct {
			URL string `json:"url"`
		} `json:"thumbnail"`
		Source struct {
			MediaType string `json:"media_type"`
			Name      string `json:"name"`
			Size      string `json:"size"`
		} `json:"source"`
		Video struct {
			Duration float64 `json:"duration"`
			Height   int     `json:"height"`
		} `json:"video"`
		Audio struct {
			Duration float64 `json:"duration"`
		} `json:"audio"`
		Fee *struct {
			Amount string `json:"amount"`
		} `json:"fee"`
	} `json:"value"`
	Error *struct {
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"error"`
}

// lbryURI maps /@channel:5/name:e, /name:claimid and /$/embed/... to lbry://, "" for listings.
func lbryURI(u *url.URL) string {
	p := pathParts(u)
	if len(p) >= 2 && p[0] == "$" && (p[1] == "embed" || p[1] == "download") {
		p = p[2:]
	}
	if len(p) == 0 || p[0] == "$" || (len(p) == 1 && strings.HasPrefix(p[0], "@")) || len(p) > 2 {
		return ""
	}
	if len(p) == 2 && !strings.HasPrefix(p[0], "@") {
		// /$/embed/name/claimid
		p = []string{p[0] + ":" + p[1]}
	}
	for i, s := range p {
		if v, err := url.PathUnescape(s); err == nil {
			s = v
		}
		p[i] = strings.Replace(s, ":", "#", 1)
	}
	return "lbry://" + strings.Join(p, "/")
}

func (s odysee) call(ctx context.Context, env *plan.Env, method string, params, out any) error {
	var resp struct {
		Result any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	resp.Result = out
	if err := postJSON(ctx, env, odyseeAPI+"?m="+method, nil,
		map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": 1}, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("%w: %s", media.ErrUnavailable, resp.Error.Message)
	}
	return nil
}

func (s odysee) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	uri := lbryURI(u)
	if uri == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	claims := map[string]lbryClaim{}
	if err := s.call(ctx, env, "resolve", map[string]any{"urls": []string{uri}}, &claims); err != nil {
		return nil, err
	}
	c, ok := claims[uri]
	if !ok || c.Error != nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post was removed or never existed."))
	}
	if c.ValueType != "stream" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	if c.Value.Fee != nil && c.Value.Fee.Amount != "" && c.Value.Fee.Amount != "0" {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This video has to be bought first."))
	}
	var got struct {
		StreamingURL string `json:"streaming_url"`
		Error        string `json:"error"`
	}
	if err := s.call(ctx, env, "get", map[string]any{"uri": first(c.PermanentURL, uri), "save_file": false}, &got); err != nil {
		return nil, err
	}
	if got.StreamingURL == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, first(got.Error, i18n.T("This video is not ready to download.")))
	}
	page := "https://odysee.com/" + strings.ReplaceAll(strings.TrimPrefix(first(c.CanonicalURL, uri), "lbry://"), "#", ":")
	author := ""
	if c.Channel != nil {
		author = first(c.Channel.Value.Title, c.Channel.Name)
	}
	v := c.Value
	size, err := strconv.ParseInt(v.Source.Size, 10, 64)
	if err != nil || size <= 0 {
		size = -1
	}
	ext := strings.TrimPrefix(path.Ext(first(v.Source.Name, got.StreamingURL)), ".")
	out := &media.Media{Kind: plan.KindMedia, URL: page, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(v.Title, c.Name), Author: author, Thumb: v.Thumbnail.URL}
	switch v.StreamType {
	case "video":
		out.Duration = v.Video.Duration
		f := fileVideo("original", got.StreamingURL, first(ext, "mp4"), v.Video.Height, 0, 0, size, "", nil)
		f.Label = i18n.T("Original")
		if h := heightLabel(v.Video.Height, 0); h != "" {
			f.Label = i18n.Tf("%s original", h)
		}
		out.Formats = append(out.Formats, f)
		// the transcoded ladder sits beside the stream under its sd hash prefix
		master := strings.TrimSuffix(got.StreamingURL, path.Ext(got.StreamingURL)) + "/master.m3u8"
		if vs, err := hls.Variants(ctx, env, master, nil); err == nil {
			out.Formats = append(out.Formats, media.HLSFormats(vs, nil)...)
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	case "audio":
		out.Duration = v.Audio.Duration
		out.Formats = append(out.Formats, fileAudio("original", got.StreamingURL, first(ext, "mp3"), 0, size, "", nil))
	default:
		name := media.SafeName(first(v.Source.Name, v.Title, c.Name), "")
		return &media.Media{Kind: plan.KindFile, URL: got.StreamingURL, Source: out.Source, Title: out.Title, Author: author,
			Thumb: out.Thumb, Name: name, Size: size, Mime: v.Source.MediaType, Resumable: true}, nil
	}
	out.Default = out.Formats[0].ID
	return out, nil
}

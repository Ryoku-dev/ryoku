package services

import (
	"context"
	"fmt"
	"net/url"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var bskyAPI = "https://public.api.bsky.app/xrpc"

type bluesky struct{}

func init() { media.Register(bluesky{}) }

func (bluesky) ID() string   { return "bluesky" }
func (bluesky) Name() string { return "Bluesky" }

func (bluesky) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "bsky.app") }

type bskyEmbed struct {
	Type      string `json:"$type"`
	Playlist  string `json:"playlist"`
	Thumbnail string `json:"thumbnail"`
	Images    []struct {
		Thumb    string `json:"thumb"`
		Fullsize string `json:"fullsize"`
	} `json:"images"`
	External *struct {
		URI   string `json:"uri"`
		Title string `json:"title"`
		Thumb string `json:"thumb"`
	} `json:"external"`
	Media *bskyEmbed `json:"media"`
}

func (s bluesky) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	p := pathParts(u)
	if len(p) < 4 || p[0] != "profile" || p[2] != "post" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single post can be downloaded."))
	}
	q := url.Values{
		"uri":          {"at://" + p[1] + "/app.bsky.feed.post/" + p[3]},
		"depth":        {"0"},
		"parentHeight": {"0"},
	}
	r, err := fetch(ctx, env, request{url: bskyAPI + "/app.bsky.feed.getPostThread?" + q.Encode(),
		header: hdr("Accept", "application/json"), accept: []int{400}})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Error  string `json:"error"`
		Thread struct {
			Type string `json:"$type"`
			Post *struct {
				Author struct {
					Handle      string `json:"handle"`
					DisplayName string `json:"displayName"`
				} `json:"author"`
				Record struct {
					Text string `json:"text"`
				} `json:"record"`
				Embed *bskyEmbed `json:"embed"`
			} `json:"post"`
		} `json:"thread"`
	}
	if err := decode(r.body, &resp); err != nil {
		return nil, err
	}
	if r.status == 400 || resp.Thread.Post == nil {
		if resp.Thread.Type == "app.bsky.feed.defs#blockedPost" {
			return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This post is blocked."))
		}
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post does not exist."))
	}
	post := resp.Thread.Post
	src := media.Source{ID: s.ID(), Name: s.Name()}
	author := first(post.Author.DisplayName, post.Author.Handle)
	title := oneLine(post.Record.Text, 90)
	if title == "" {
		title = i18n.Tf("Post by %s", first(author, src.Name))
	}
	e := post.Embed
	if e != nil && e.Media != nil {
		e = e.Media
	}
	if e == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video or picture."))
	}
	page := "https://bsky.app/profile/" + p[1] + "/post/" + p[3]

	switch e.Type {
	case "app.bsky.embed.video#view":
		vs, err := hls.Variants(ctx, env, e.Playlist, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
		}
		out := &media.Media{Kind: plan.KindMedia, URL: page, Source: src, Title: title, Author: author, Thumb: e.Thumbnail,
			Formats: media.HLSFormats(vs, nil)}
		out.Default = best(out.Formats)
		return out, nil
	case "app.bsky.embed.images#view":
		if len(e.Images) == 1 {
			return photoFile(src, e.Images[0].Fullsize, title, author, nil), nil
		}
		out := &media.Media{Kind: plan.KindMedia, URL: page, Source: src, Title: title, Author: author}
		for i, img := range e.Images {
			out.Items = append(out.Items, media.Item{Title: i18n.Tf("Picture %d", i+1), Thumb: img.Thumb, Kind: "photo", URL: img.Fullsize})
		}
		if len(out.Items) > 0 {
			out.Thumb = out.Items[0].Thumb
		}
		out.Note = i18n.Tf("%d items in this post", len(out.Items))
		return out, nil
	case "app.bsky.embed.external#view":
		// GIFs from the composer are Tenor links
		if e.External != nil && extOf(e.External.URI, "") == "gif" {
			m := photoFile(src, e.External.URI, title, author, nil)
			m.Mime = "image/gif"
			return m, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video or picture."))
}

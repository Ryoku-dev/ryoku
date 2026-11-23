package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

// linkedin posts carry their sources in data-sources, and listings are refused before yt-dlp sees them.
type linkedin struct{}

func init() { media.Register(linkedin{}) }

func (linkedin) ID() string   { return "linkedin" }
func (linkedin) Name() string { return "LinkedIn" }

func (linkedin) Match(u *url.URL) bool {
	if !media.HostIs(u.Hostname(), "linkedin.com") {
		return false
	}
	return linkedinPost(u) || linkedinListing(u)
}

func linkedinPost(u *url.URL) bool {
	p := strings.TrimSuffix(u.Path, "/")
	return strings.HasPrefix(p, "/posts/") || strings.HasPrefix(p, "/feed/update/") || strings.HasPrefix(p, "/embed/feed/update/")
}

// linkedinListing reports LinkedIn's listing pages.
// ported from prism-download-manager crates/prism-core/src/manager/capture.rs (MIT)
func linkedinListing(u *url.URL) bool {
	if strings.HasPrefix(u.Path, "/feed/update/") {
		return false
	}
	first := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)[0]
	switch first {
	case "", "feed", "company", "in", "school", "showcase", "groups", "search", "mynetwork", "jobs",
		"notifications", "messaging", "my-items":
		return true
	}
	return false
}

var (
	linkedinSourcesRe = regexp.MustCompile(`data-sources="([^"]*)"`)
	linkedinPosterRe  = regexp.MustCompile(`data-poster-url="([^"]*)"`)
	linkedinHeightRe  = regexp.MustCompile(`mp4-(\d{3,4})p-(\d+)fp`)
)

func (s linkedin) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	if !linkedinPost(u) {
		return nil, media.Settled(fmt.Errorf("%w: %s", media.ErrUnsupported,
			i18n.T("This LinkedIn page lists posts. Open the post itself (… › Copy link to post) and try that link.")))
	}
	page := url.URL{Scheme: "https", Host: "www.linkedin.com", Path: u.Path}
	r, err := getBody(ctx, env, page.String(), navigate())
	if err != nil {
		return nil, err
	}
	if p := r.final.Path; strings.Contains(p, "authwall") || strings.HasPrefix(p, "/login") || strings.HasPrefix(p, "/uas/") {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("LinkedIn shows this post only to signed-in members."))
	}
	m := linkedinSourcesRe.FindSubmatch(r.body)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video."))
	}
	var srcs []struct {
		Src     string `json:"src"`
		Type    string `json:"type"`
		Bitrate int    `json:"data-bitrate"`
	}
	if err := json.Unmarshal([]byte(unescapeHTML(string(m[1]))), &srcs); err != nil {
		return nil, fmt.Errorf("%w: unexpected response: %v", media.ErrUnavailable, err)
	}
	meta := metaTags(r.body)
	author, _, _ := strings.Cut(meta["og:title"], " | ")
	thumb := ""
	if pm := linkedinPosterRe.FindSubmatch(r.body); pm != nil {
		thumb = unescapeHTML(string(pm[1]))
	}
	out := &media.Media{Kind: plan.KindMedia, URL: page.String(), Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(oneLine(meta["og:description"], 80), i18n.T("LinkedIn video")), Author: strings.TrimSpace(author), Thumb: thumb}
	top, topRate := "", -1
	for i, src := range srcs {
		if src.Src == "" || (src.Type != "" && !strings.HasPrefix(src.Type, "video/")) {
			continue
		}
		h, fps := 0, 0
		id := "original"
		if hm := linkedinHeightRe.FindStringSubmatch(src.Src); hm != nil {
			h, _ = strconv.Atoi(hm[1])
			fps, _ = strconv.Atoi(hm[2])
			id = "mp4-" + hm[1] + "p"
		}
		if hasFormat(out, id) {
			id += "-" + strconv.Itoa(i)
		}
		f := fileVideo(id, src.Src, "mp4", h, fps, src.Bitrate, -1, "h264,aac", nil)
		if h == 0 {
			f.Label = i18n.T("Original")
		}
		out.Formats = append(out.Formats, f)
		if src.Bitrate > topRate {
			top, topRate = id, src.Bitrate
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video."))
	}
	out.Default = top
	return out, nil
}

func hasFormat(m *media.Media, id string) bool {
	for _, f := range m.Formats {
		if f.ID == id {
			return true
		}
	}
	return false
}

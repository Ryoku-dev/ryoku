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

var facebookBase = "https://www.facebook.com"

type facebook struct{}

func init() { media.Register(facebook{}) }

func (facebook) ID() string   { return "facebook" }
func (facebook) Name() string { return "Facebook" }

func (facebook) Match(u *url.URL) bool {
	return media.HostIs(u.Hostname(), "facebook.com", "fb.watch", "fb.com")
}

var (
	fbVideoPathRe = regexp.MustCompile(`/(?:videos|reel|reels)/(?:[^/]+/)?(\d{6,})`)
	fbHDRe        = regexp.MustCompile(`"browser_native_hd_url":("(?:[^"\\]|\\.)*")`)
	fbSDRe        = regexp.MustCompile(`"browser_native_sd_url":("(?:[^"\\]|\\.)*")`)
	fbDurRe       = regexp.MustCompile(`"playable_duration_in_ms":(\d+)`)
	fbTitleRe     = regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	fbLoginRe     = regexp.MustCompile(`id="login_form"|/login/\?next=`)
	// "744K views · 2.3K shares | " ahead of the caption
	fbCountsRe = regexp.MustCompile(`^[^|]*\b(?:views|reactions|shares)\b[^|]*\|\s*`)
)

func (s facebook) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	page := u
	if media.HostIs(u.Hostname(), "fb.watch") || strings.HasPrefix(u.Path, "/share/") {
		to, err := follow(ctx, env, u.String())
		if err != nil {
			return nil, err
		}
		page = to
	}
	id := page.Query().Get("v")
	if id == "" {
		if m := fbVideoPathRe.FindStringSubmatch(page.Path); m != nil {
			id = m[1]
		}
	}
	if id == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	target := facebookBase + page.EscapedPath()
	if page.RawQuery != "" {
		target += "?" + page.RawQuery
	}
	r, err := fetch(ctx, env, request{url: target, header: navigate()})
	if err != nil {
		return nil, err
	}
	hd, sd := fbString(fbHDRe, r.body), fbString(fbSDRe, r.body)
	if hd == "" && sd == "" {
		if fbLoginRe.Match(r.body) || strings.Contains(r.final.Path, "/login") {
			return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This video is only visible to signed-in accounts."))
		}
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video is unavailable or not public."))
	}
	meta := metaTags(r.body)
	title := first(meta["og:title"], meta["twitter:title"])
	if title == "" {
		if m := fbTitleRe.FindSubmatch(r.body); m != nil {
			title = unescapeHTML(string(m[1]))
		}
	}
	title = fbCountsRe.ReplaceAllString(strings.TrimSuffix(strings.TrimSpace(title), " | Facebook"), "")
	if title == "" || title == "Facebook" || title == "Video" {
		title = i18n.Tf("Facebook video %s", id)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: facebookBase + "/watch/?v=" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: oneLine(title, 120), Thumb: meta["og:image"], Headers: map[string]string{"User-Agent": browserUA}}
	if m := fbDurRe.FindSubmatch(r.body); m != nil {
		ms, _ := strconv.Atoi(string(m[1]))
		out.Duration = float64(ms) / 1000
	}
	if hd != "" {
		f := fileVideo("hd", hd, "mp4", 0, 0, 0, 0, "h264,aac", nil)
		f.Label = "HD"
		out.Formats = append(out.Formats, f)
	}
	if sd != "" {
		f := fileVideo("sd", sd, "mp4", 0, 0, 0, 0, "h264,aac", nil)
		f.Label = "SD"
		out.Formats = append(out.Formats, f)
	}
	out.Default = out.Formats[0].ID
	return out, nil
}

func fbString(re *regexp.Regexp, page []byte) string {
	m := re.FindSubmatch(page)
	if m == nil {
		return ""
	}
	var s string
	if json.Unmarshal(m[1], &s) != nil {
		return ""
	}
	return s
}

package services

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var xSyndication = "https://cdn.syndication.twimg.com/tweet-result"

type twitter struct{}

func init() { media.Register(twitter{}) }

func (twitter) ID() string   { return "x" }
func (twitter) Name() string { return "X" }

func (twitter) Match(u *url.URL) bool {
	return media.HostIs(u.Hostname(), "x.com", "twitter.com", "t.co")
}

var xStatusRe = regexp.MustCompile(`^/(?:[^/]+/status(?:es)?|i/(?:web/)?status)/(\d+)(?:/(video|photo)/(\d+))?`)

type xTweet struct {
	Typename     string    `json:"__typename"`
	IDStr        string    `json:"id_str"`
	Text         string    `json:"text"`
	User         xUser     `json:"user"`
	MediaDetails []xMedia  `json:"mediaDetails"`
	QuotedTweet  *xTweet   `json:"quoted_tweet"`
	Tombstone    *struct{} `json:"tombstone"`
}

type xUser struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type xMedia struct {
	Type          string `json:"type"`
	MediaURLHTTPS string `json:"media_url_https"`
	OriginalInfo  struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"original_info"`
	VideoInfo struct {
		DurationMillis int `json:"duration_millis"`
		Variants       []struct {
			Bitrate     int    `json:"bitrate"`
			ContentType string `json:"content_type"`
			URL         string `json:"url"`
		} `json:"variants"`
	} `json:"video_info"`
}

var xDimRe = regexp.MustCompile(`/(\d+)x(\d+)/`)

func (s twitter) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	if media.HostIs(u.Hostname(), "t.co") {
		to, err := follow(ctx, env, u.String(), "x.com", "twitter.com")
		if err != nil {
			return nil, err
		}
		if !media.HostIs(to.Hostname(), "x.com", "twitter.com") {
			return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("This short link does not lead to a post."))
		}
		u = to
	}
	m := xStatusRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single post can be downloaded."))
	}
	id, pick := m[1], 0
	if m[3] != "" {
		pick, _ = strconv.Atoi(m[3])
	}

	q := url.Values{"id": {id}, "token": {xToken(id)}, "lang": {"en"}}
	var t xTweet
	r, err := fetch(ctx, env, request{url: xSyndication + "?" + q.Encode(), header: hdr("Accept", "application/json"), accept: []int{404}})
	if err != nil {
		return nil, err
	}
	if r.status == 404 || len(r.body) < 3 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post does not exist or its account is protected."))
	}
	if err := decode(r.body, &t); err != nil {
		return nil, err
	}
	if t.Typename == "TweetTombstone" || t.Tombstone != nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post is not available."))
	}
	src := media.Source{ID: s.ID(), Name: s.Name()}
	title := oneLine(xLinkRe.ReplaceAllString(t.Text, ""), 90)
	author := first(t.User.Name, t.User.ScreenName)
	if title == "" {
		title = i18n.Tf("Post by %s", author)
	}
	page := "https://x.com/" + first(t.User.ScreenName, "i") + "/status/" + id

	all := t.MediaDetails
	if len(all) == 0 && t.QuotedTweet != nil {
		all = t.QuotedTweet.MediaDetails
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video or picture."))
	}
	if pick > 0 && pick <= len(all) {
		all = all[pick-1 : pick]
	}

	if len(all) == 1 && all[0].Type == "photo" {
		return photoFile(src, xOrig(all[0].MediaURLHTTPS), title, author, nil), nil
	}

	out := &media.Media{Kind: plan.KindMedia, URL: page, Source: src, Title: title, Author: author}
	var videoIdx, photoIdx int
	for i, md := range all {
		kind := "video"
		itemURL := ""
		switch md.Type {
		case "photo":
			kind = "photo"
			photoIdx++
			itemURL = xOrig(md.MediaURLHTTPS)
		case "animated_gif":
			kind = "gif"
			videoIdx++
			itemURL = page + "/video/" + strconv.Itoa(i+1)
		default:
			videoIdx++
			itemURL = page + "/video/" + strconv.Itoa(i+1)
		}
		if len(all) > 1 {
			label := i18n.Tf("Video %d", videoIdx)
			if kind == "photo" {
				label = i18n.Tf("Picture %d", photoIdx)
			}
			out.Items = append(out.Items, media.Item{Title: label, Thumb: md.MediaURLHTTPS, Kind: kind, URL: itemURL})
		}
		if kind == "photo" || len(out.Formats) > 0 {
			continue
		}
		out.Thumb = md.MediaURLHTTPS
		out.Duration = float64(md.VideoInfo.DurationMillis) / 1000
		fs, err := xFormats(ctx, env, md)
		if err != nil {
			return nil, err
		}
		out.Formats = fs
	}
	if out.Thumb == "" {
		out.Thumb = all[0].MediaURLHTTPS
	}
	if len(out.Items) > 1 {
		out.Note = i18n.Tf("%d items in this post", len(out.Items))
	}
	out.Default = best(out.Formats)
	return out, nil
}

var xLinkRe = regexp.MustCompile(`\s*https://t\.co/\w+`)

func xFormats(ctx context.Context, env *plan.Env, md xMedia) ([]media.Format, error) {
	var fs []media.Format
	var playlist string
	for _, v := range md.VideoInfo.Variants {
		switch v.ContentType {
		case "video/mp4":
			h := md.OriginalInfo.Height
			if d := xDimRe.FindStringSubmatch(v.URL); d != nil {
				h, _ = strconv.Atoi(d[2])
			}
			fs = append(fs, fileVideo("mp4-"+strconv.Itoa(len(fs)), v.URL, "mp4", h, 0, v.Bitrate, 0, "avc1", nil))
		case "application/x-mpegURL":
			playlist = v.URL
		}
	}
	if len(fs) > 0 || playlist == "" {
		return fs, nil
	}
	vs, err := hls.Variants(ctx, env, playlist, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	return media.HLSFormats(vs, nil), nil
}

// xOrig asks pbs.twimg.com for the picture as uploaded.
func xOrig(u string) string {
	if u == "" || strings.Contains(u, "?") {
		return u
	}
	return u + "?name=orig"
}

// xToken is the syndication token X's own embed computes:
// (id / 1e15 * pi) in base 36, zeros and the point removed.
func xToken(id string) string {
	n, err := strconv.ParseFloat(id, 64)
	if err != nil {
		return ""
	}
	return strings.NewReplacer("0", "", ".", "").Replace(jsRadix36(n / 1e15 * math.Pi))
}

// jsRadix36 matches JavaScript's Number.prototype.toString(36) for positive
// finite values, fractional digits included.
func jsRadix36(v float64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return "0"
	}
	integer, fraction := math.Modf(v)
	delta := math.Max(math.Nextafter(0, 1), (math.Nextafter(v, math.Inf(1))-v)/2)
	var frac []int
	for fraction >= delta {
		delta *= 36
		d, f := math.Modf(fraction * 36)
		fraction = f
		frac = append(frac, int(d))
		round := fraction > 0.5 || (fraction == 0.5 && int(d)&1 == 1)
		if round && fraction+delta > 1 {
			carried := false
			for i := len(frac) - 1; i >= 0; i-- {
				if frac[i]+1 < 36 {
					frac[i]++
					carried = true
					break
				}
				frac = frac[:i]
			}
			if !carried {
				integer++
			}
			break
		}
	}
	n := uint64(integer)
	var ib []byte
	for {
		ib = append([]byte{digits[n%36]}, ib...)
		n /= 36
		if n == 0 {
			break
		}
	}
	if len(frac) == 0 {
		return string(ib)
	}
	b := append(ib, '.')
	for _, d := range frac {
		b = append(b, digits[d])
	}
	return string(b)
}

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

// bandcamp pages carry mp3 stream links in data-tralbum, and its bot check lets only plain clients through.
type bandcamp struct{}

func init() { media.Register(bandcamp{}) }

func (bandcamp) ID() string   { return "bandcamp" }
func (bandcamp) Name() string { return "Bandcamp" }

func (bandcamp) Match(u *url.URL) bool {
	p := pathParts(u)
	return media.HostIs(u.Hostname(), "bandcamp.com") && len(p) >= 2 && (p[0] == "track" || p[0] == "album")
}

var tralbumRe = regexp.MustCompile(`data-tralbum="([^"]*)"`)

type bandcampTrack struct {
	ID        int64             `json:"id"`
	Title     string            `json:"title"`
	File      map[string]string `json:"file"`
	Duration  float64           `json:"duration"`
	TitleLink string            `json:"title_link"`
}

type bandcampTralbum struct {
	Artist   string `json:"artist"`
	ItemType string `json:"item_type"`
	ArtID    int64  `json:"art_id"`
	URL      string `json:"url"`
	Current  struct {
		Title string `json:"title"`
	} `json:"current"`
	TrackInfo []bandcampTrack `json:"trackinfo"`
}

var bandcampRateRe = regexp.MustCompile(`-(\d+)$`)

func (s bandcamp) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	page := url.URL{Scheme: "https", Host: u.Host, Path: u.Path}
	r, err := getBody(ctx, env, page.String(), hdr("User-Agent", plainUA, "Accept", "text/html"))
	if err != nil {
		return nil, err
	}
	if media.ChallengePage(r.header, r.body) {
		return nil, media.BrowserCheckError()
	}
	m := tralbumRe.FindSubmatch(r.body)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This page has no player data."))
	}
	var t bandcampTralbum
	if err := json.Unmarshal([]byte(unescapeHTML(string(m[1]))), &t); err != nil {
		return nil, fmt.Errorf("%w: unexpected response: %v", media.ErrUnavailable, err)
	}
	meta := metaTags(r.body)
	thumb := meta["og:image"]
	if thumb == "" && t.ArtID > 0 {
		thumb = fmt.Sprintf("https://f4.bcbits.com/img/a%010d_10.jpg", t.ArtID)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: page.String(), Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(t.Current.Title, meta["og:title"]), Author: t.Artist, Thumb: thumb}

	if t.ItemType == "album" {
		for _, tr := range t.TrackInfo {
			if len(tr.File) == 0 || tr.TitleLink == "" {
				continue
			}
			link, err := page.Parse(tr.TitleLink)
			if err != nil {
				continue
			}
			out.Items = append(out.Items, media.Item{Title: tr.Title, Kind: "audio", URL: link.String(), Thumb: thumb})
			out.Duration += tr.Duration
		}
		if len(out.Items) == 0 {
			return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("Nothing on this album plays without buying it."))
		}
		return out, nil
	}

	if len(t.TrackInfo) == 0 || len(t.TrackInfo[0].File) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This track does not play without buying it."))
	}
	tr := t.TrackInfo[0]
	// a track page's trackinfo title can carry "artist - " in front
	out.Title = first(out.Title, tr.Title)
	out.Duration = tr.Duration
	keys := make([]string, 0, len(tr.File))
	for k := range tr.File {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if tr.File[k] == "" || !strings.HasPrefix(k, "mp3") {
			continue
		}
		kbps := 0
		if mm := bandcampRateRe.FindStringSubmatch(k); mm != nil {
			kbps, _ = strconv.Atoi(mm[1])
		}
		out.Formats = append(out.Formats, fileAudio(k, tr.File[k], "mp3", kbps*1000, -1, "mp3", nil))
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This track does not play without buying it."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

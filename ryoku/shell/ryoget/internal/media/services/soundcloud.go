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

var (
	soundcloudBase = "https://soundcloud.com"
	soundcloudAPI  = "https://api-v2.soundcloud.com"
)

type soundcloud struct{}

func init() { media.Register(soundcloud{}) }

func (soundcloud) ID() string   { return "soundcloud" }
func (soundcloud) Name() string { return "SoundCloud" }

func (soundcloud) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "soundcloud.com") }

type scTrack struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	PermalinkURL string  `json:"permalink_url"`
	ArtworkURL   string  `json:"artwork_url"`
	Duration     float64 `json:"duration"`
	Policy       string  `json:"policy"`
	Streamable   *bool   `json:"streamable"`
	TrackAuth    string  `json:"track_authorization"`
	User         struct {
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	Media struct {
		Transcodings []struct {
			URL     string `json:"url"`
			Preset  string `json:"preset"`
			Snipped bool   `json:"snipped"`
			Format  struct {
				Protocol string `json:"protocol"`
				MimeType string `json:"mime_type"`
			} `json:"format"`
		} `json:"transcodings"`
	} `json:"media"`
}

type scPlaylist struct {
	Title        string  `json:"title"`
	PermalinkURL string  `json:"permalink_url"`
	ArtworkURL   string  `json:"artwork_url"`
	Duration     float64 `json:"duration"`
	User         struct {
		Username string `json:"username"`
	} `json:"user"`
	Tracks []scTrack `json:"tracks"`
}

var scClientIDRe = regexp.MustCompile(`client_id\s*:\s*"([0-9A-Za-z]{20,40})"`)
var scAssetRe = regexp.MustCompile(`<script[^>]+src="(https://a-v2\.sndcdn\.com/assets/[^"]+\.js)"`)

func (s soundcloud) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	page := *u
	page.Host = "soundcloud.com"
	page.Scheme = "https"
	if media.HostIs(u.Hostname(), "on.soundcloud.com") {
		to, err := follow(ctx, env, u.String(), "soundcloud.com", "m.soundcloud.com")
		if err != nil {
			return nil, err
		}
		page = *to
		page.Host = "soundcloud.com"
	}
	page.RawQuery, page.Fragment = "", ""
	r, err := fetch(ctx, env, request{url: soundcloudBase + page.EscapedPath(), header: navigate()})
	if err != nil {
		return nil, err
	}
	var hydration []struct {
		Kind string          `json:"hydratable"`
		Data json.RawMessage `json:"data"`
	}
	raw := scriptJSON(r.body, "window.__sc_hydration")
	if raw == nil || decode(raw, &hydration) != nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("SoundCloud did not send the track."))
	}
	clientID := ""
	var track *scTrack
	var set *scPlaylist
	for _, h := range hydration {
		switch h.Kind {
		case "apiClient":
			var c struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(h.Data, &c) == nil {
				clientID = c.ID
			}
		case "sound":
			track = &scTrack{}
			if err := decode(h.Data, track); err != nil {
				return nil, err
			}
		case "playlist":
			set = &scPlaylist{}
			if err := decode(h.Data, set); err != nil {
				return nil, err
			}
		}
	}
	if track == nil && set == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a track or a playlist can be downloaded."))
	}
	if clientID == "" {
		if clientID, err = scScrapeClientID(ctx, env, r.body); err != nil {
			return nil, err
		}
	}
	if set != nil {
		return s.playlist(ctx, env, set, clientID)
	}
	return s.track(ctx, env, track, clientID)
}

// scScrapeClientID finds the web app's client id in its script bundles, for
// pages that do not hydrate it.
func scScrapeClientID(ctx context.Context, env *plan.Env, page []byte) (string, error) {
	assets := scAssetRe.FindAllSubmatch(page, -1)
	for i := len(assets) - 1; i >= 0; i-- {
		r, err := getBody(ctx, env, string(assets[i][1]), nil)
		if err != nil {
			continue
		}
		if m := scClientIDRe.FindSubmatch(r.body); m != nil {
			return string(m[1]), nil
		}
	}
	return "", fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("SoundCloud did not hand out a client id."))
}

func (s soundcloud) track(ctx context.Context, env *plan.Env, t *scTrack, clientID string) (*media.Media, error) {
	if t.Streamable != nil && !*t.Streamable {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This track cannot be played outside SoundCloud."))
	}
	out := &media.Media{
		Kind: plan.KindMedia, URL: t.PermalinkURL, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: t.Title, Author: t.User.Username, Thumb: scArtwork(first(t.ArtworkURL, t.User.AvatarURL)),
		Duration: t.Duration / 1000,
	}
	if t.Policy == "SNIP" {
		out.Note = i18n.T("SoundCloud only offers a 30 second preview of this track.")
	}
	seen := map[string]bool{}
	// progressive files first: one plain file beats a playlist of pieces
	for _, progressive := range []bool{true, false} {
		for _, tc := range t.Media.Transcodings {
			if (tc.Format.Protocol == "progressive") != progressive || strings.HasPrefix(tc.Preset, "abr") {
				continue
			}
			if tc.Format.Protocol != "progressive" && tc.Format.Protocol != "hls" {
				continue
			}
			ext, codec := scCodec(tc.Preset, tc.Format.MimeType)
			key := codec + strconv.Itoa(scBitrate(tc.Preset))
			if seen[key] {
				continue
			}
			var loc struct {
				URL string `json:"url"`
			}
			q := url.Values{"client_id": {clientID}}
			if t.TrackAuth != "" {
				q.Set("track_authorization", t.TrackAuth)
			}
			if err := getJSON(ctx, env, tc.URL+"?"+q.Encode(), nil, &loc); err != nil || loc.URL == "" {
				continue
			}
			seen[key] = true
			f := fileAudio(tc.Preset+"-"+tc.Format.Protocol, loc.URL, ext, scBitrate(tc.Preset), 0, codec, nil)
			if tc.Format.Protocol == "hls" {
				f.Parts[0].Kind = plan.PartHLS
				f.Parts[0].Ext = ""
			}
			out.Formats = append(out.Formats, f)
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This track has no stream SoundCloud will share."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func (s soundcloud) playlist(ctx context.Context, env *plan.Env, p *scPlaylist, clientID string) (*media.Media, error) {
	// a set's page carries the first few tracks in full and only ids after that
	var missing []string
	for _, t := range p.Tracks {
		if t.PermalinkURL == "" {
			missing = append(missing, strconv.FormatInt(t.ID, 10))
		}
	}
	full := map[int64]scTrack{}
	for len(missing) > 0 {
		n := min(len(missing), 50)
		var batch []scTrack
		q := url.Values{"ids": {strings.Join(missing[:n], ",")}, "client_id": {clientID}}
		if err := getJSON(ctx, env, soundcloudAPI+"/tracks?"+q.Encode(), nil, &batch); err != nil {
			return nil, err
		}
		for _, t := range batch {
			full[t.ID] = t
		}
		missing = missing[n:]
	}
	out := &media.Media{
		Kind: plan.KindMedia, URL: p.PermalinkURL, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: p.Title, Author: p.User.Username, Duration: p.Duration / 1000,
	}
	for _, t := range p.Tracks {
		if t.PermalinkURL == "" {
			t = full[t.ID]
		}
		if t.PermalinkURL == "" {
			continue
		}
		out.Items = append(out.Items, media.Item{Title: t.Title, Thumb: scArtwork(t.ArtworkURL), Kind: "audio", URL: t.PermalinkURL})
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This playlist is empty."))
	}
	out.Thumb = first(scArtwork(p.ArtworkURL), out.Items[0].Thumb)
	out.Note = i18n.Tf("%d tracks in this playlist", len(out.Items))
	return out, nil
}

// scArtwork asks for the 500 px cover instead of the 100 px default.
func scArtwork(u string) string { return strings.Replace(u, "-large.", "-t500x500.", 1) }

func scCodec(preset, mime string) (ext, codec string) {
	switch {
	case strings.HasPrefix(preset, "mp3") || strings.Contains(mime, "mpeg"):
		return "mp3", "mp3"
	case strings.HasPrefix(preset, "opus") || strings.Contains(mime, "opus"):
		return "opus", "opus"
	}
	return "m4a", "aac"
}

var scKbpsRe = regexp.MustCompile(`_(\d+)k`)

func scBitrate(preset string) int {
	if m := scKbpsRe.FindStringSubmatch(preset); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n * 1000
	}
	switch {
	case strings.HasPrefix(preset, "mp3"):
		return 128000
	case strings.HasPrefix(preset, "opus"):
		return 64000
	}
	return 0
}

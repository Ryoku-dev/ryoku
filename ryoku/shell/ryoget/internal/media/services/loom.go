package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var loomBase = "https://www.loom.com"

type loom struct{}

func init() { media.Register(loom{}) }

func (loom) ID() string   { return "loom" }
func (loom) Name() string { return "Loom" }

func (loom) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "loom.com") }

var loomIDRe = regexp.MustCompile(`^/(?:share|embed|v)/(?:[\w-]+-)?([0-9a-f]{32})`)

const loomQuery = `query GetVideoSSR($videoId: ID!, $password: String) { getVideo(id: $videoId, password: $password) { __typename ... on PrivateVideo { id status message } ... on VideoPasswordMissingOrIncorrect { id message } ... on RegularUserVideo { id name owner { display_name } video_properties { duration height width } } } }`

type loomURL struct {
	URL   string `json:"url"`
	Creds struct {
		Policy    string `json:"Policy"`
		Signature string `json:"Signature"`
		KeyPairID string `json:"Key-Pair-Id"`
	} `json:"part_credentials"`
}

func (s loom) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := loomIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := m[1]
	var meta struct {
		Data struct {
			Video *struct {
				Typename string `json:"__typename"`
				Name     string `json:"name"`
				Owner    *struct {
					DisplayName string `json:"display_name"`
				} `json:"owner"`
				Props *struct {
					Duration float64 `json:"duration"`
					Height   int     `json:"height"`
				} `json:"video_properties"`
			} `json:"getVideo"`
		} `json:"data"`
	}
	gql := hdr("Origin", loomBase, "apollographql-client-name", "web", "apollographql-client-version", "45a5bd4",
		"x-loom-request-source", "loom_web_45a5bd4")
	body := map[string]any{"operationName": "GetVideoSSR", "variables": map[string]any{"videoId": id, "password": nil}, "query": loomQuery}
	if err := postJSON(ctx, env, loomBase+"/graphql", gql, body, &meta); err != nil {
		return nil, err
	}
	v := meta.Data.Video
	switch {
	case v == nil:
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video does not exist."))
	case v.Typename == "PrivateVideo" || v.Typename == "VideoPasswordMissingOrIncorrect":
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This video is private or needs a password."))
	}
	out := &media.Media{Kind: plan.KindMedia, URL: loomBase + "/share/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(v.Name, id)}
	height := 0
	if v.Owner != nil {
		out.Author = v.Owner.DisplayName
	}
	if v.Props != nil {
		out.Duration, height = v.Props.Duration, v.Props.Height
	}

	anon := make([]byte, 16)
	_, _ = rand.Read(anon)
	req := map[string]any{"anonID": hex.EncodeToString(anon), "deviceID": nil, "force_original": false, "password": nil}
	for _, ep := range []string{"transcoded-url", "raw-url"} {
		var r loomURL
		resp, err := fetch(ctx, env, request{method: http.MethodPost, url: loomBase + "/api/campaigns/sessions/" + id + "/" + ep,
			header: hdr("Accept", "application/json", "Content-Type", "application/json"), body: mustJSON(req), accept: []int{204, 404}})
		if err != nil {
			return nil, err
		}
		if resp.status != 200 || decode(resp.body, &r) != nil || r.URL == "" {
			continue
		}
		switch extOf(r.URL, "") {
		case "m3u8":
			// CloudFront takes the signed policy as cookies for every playlist and segment
			h := http.Header{}
			if c := r.Creds; c.Policy != "" {
				h.Set("Cookie", "CloudFront-Policy="+c.Policy+"; CloudFront-Signature="+c.Signature+"; CloudFront-Key-Pair-Id="+c.KeyPairID)
			}
			vs, err := hls.Variants(ctx, env, r.URL, h)
			if err != nil {
				continue
			}
			out.Formats = append(out.Formats, media.HLSFormats(vs, h)...)
		case "mpd":
			// DASH manifests are left to the yt-dlp fallback
		default:
			out.Formats = append(out.Formats, fileVideo(ep, r.URL, extOf(r.URL, "mp4"), height, 0, 0, 0, "", nil))
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream Loom will share."))
	}
	out.Default = best(out.Formats)
	return out, nil
}
